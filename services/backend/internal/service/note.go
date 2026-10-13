package service

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/edgorman/webapp-template/services/backend/internal/entity"
	appv1 "github.com/edgorman/webapp-template/services/backend/internal/gen/app/v1"
	"github.com/edgorman/webapp-template/services/backend/internal/repository"
)

// noteMessage is the wire shape of a single note: what GetNote, CreateNote and UpdateNote answer
// with, and what each entry of a NotePage carries. author is the owner's username, resolved on read
// (see usernamesFor), since the uid a note records its owner by is never public.
func noteMessage(note entity.Note, author string) *appv1.Note {
	return &appv1.Note{
		Id:             note.ID,
		AuthorUsername: author,
		Title:          note.Title,
		Body:           note.Body,
		Visibility:     string(note.Visibility),
		CreatedAt:      timestamppb.New(note.CreatedAt),
		UpdatedAt:      timestamppb.New(note.UpdatedAt),
	}
}

// usernamesFor resolves the username behind each uid, looking up each distinct one once, so a feed
// dominated by one author costs one read rather than one per row. A missing profile is an empty
// username rather than a failed request: writing a note never required one.
func (s *Service) usernamesFor(ctx context.Context, uids []string) (map[string]string, error) {
	usernames := make(map[string]string)
	for _, uid := range uids {
		if _, resolved := usernames[uid]; resolved {
			continue
		}

		user, err := s.users.Get(ctx, uid)
		if errors.Is(err, repository.ErrNotFound) {
			usernames[uid] = ""
			continue
		}
		if err != nil {
			return nil, err
		}
		usernames[uid] = user.Username
	}
	return usernames, nil
}

// writeNote answers with a single note and its author.
func (s *Service) writeNote(w http.ResponseWriter, r *http.Request, status int, note entity.Note) {
	authors, err := s.usernamesFor(r.Context(), []string{note.OwnerID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeProto(w, status, noteMessage(note, authors[note.OwnerID]))
}

// decodeNoteRequest reads a note body and applies it to note through the entity's setters,
// writing the error response and returning false if it is malformed. The id, owner and timestamps
// are decided by the server, so appv1.NoteRequest carries none of them.
func decodeNoteRequest(w http.ResponseWriter, r *http.Request, note *entity.Note) bool {
	var body appv1.NoteRequest
	if !readProto(w, r, &body) {
		return false
	}

	candidate := *note
	err := candidate.SetTitle(body.GetTitle())
	if err == nil {
		err = candidate.SetBody(body.GetBody())
	}
	if err == nil {
		err = candidate.SetVisibility(entity.Visibility(body.GetVisibility()))
	}
	if err != nil {
		writeValidationError(w, err)
		return false
	}

	*note = candidate
	return true
}

// requireNotePermission loads the note the request addresses and checks the caller holds the
// named permission on it (see entity.Note.Permission), writing the error response and returning
// false otherwise.
//
// Readability is checked first whatever the action, and a caller who fails it is shown the same
// "not found" a missing note gets rather than the 403 they were really refused with, so a refusal
// never confirms that a private note exists. Only once a note is visible to the caller does a
// failed check become a 403, which reveals nothing they did not already know.
func (s *Service) requireNotePermission(w http.ResponseWriter, r *http.Request, action entity.Action) (entity.Note, bool) {
	note, err := s.notes.Get(r.Context(), r.PathValue("id"))
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, "internal error")
		return entity.Note{}, false
	}

	uid := uidFromContext(r.Context())
	if err != nil || !note.Permission(entity.ActionRead).Allows(uid) {
		writeError(w, http.StatusNotFound, "note not found")
		return entity.Note{}, false
	}
	if !note.Permission(action).Allows(uid) {
		writeError(w, http.StatusForbidden, "forbidden")
		return entity.Note{}, false
	}
	return note, true
}

// listNotesDefaultLimit is the page size ListNotes applies when a caller sends no `limit`.
const listNotesDefaultLimit = 20

// parseNoteListParams reads ListNotes' paging out of the query string: `limit` bounds the page and
// `startAfter` (an RFC3339 timestamp) continues one. A malformed one is a 400 rather than silently
// ignored.
func parseNoteListParams(r *http.Request) (repository.ListParams, error) {
	query := r.URL.Query()
	params := repository.ListParams{Limit: listNotesDefaultLimit}

	if raw := query.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit <= 0 {
			return repository.ListParams{}, errors.New("limit must be a positive integer")
		}
		params.Limit = limit
	}

	if raw := query.Get("startAfter"); raw != "" {
		startAfter, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return repository.ListParams{}, errors.New("startAfter must be an RFC3339 timestamp")
		}
		params.StartAfter = startAfter
	}

	return params, nil
}

// ListNotes returns one page of the notes the caller may read, newest first.
func (s *Service) ListNotes(w http.ResponseWriter, r *http.Request) {
	params, err := parseNoteListParams(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	notes, hasMore, err := s.notes.List(r.Context(), uidFromContext(r.Context()), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	uids := make([]string, 0, len(notes))
	for _, note := range notes {
		uids = append(uids, note.OwnerID)
	}
	authors, err := s.usernamesFor(r.Context(), uids)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Built as a non-nil slice, so an empty page is an empty JSON array rather than null.
	page := &appv1.NotePage{Notes: make([]*appv1.Note, 0, len(notes)), HasMore: hasMore}
	for _, note := range notes {
		page.Notes = append(page.Notes, noteMessage(note, authors[note.OwnerID]))
	}
	writeProto(w, http.StatusOK, page)
}

// GetNote returns a single note, provided the caller may read it.
func (s *Service) GetNote(w http.ResponseWriter, r *http.Request) {
	note, ok := s.requireNotePermission(w, r, entity.ActionRead)
	if !ok {
		return
	}
	s.writeNote(w, r, http.StatusOK, note)
}

// CreateNote makes a new note owned by the caller.
func (s *Service) CreateNote(w http.ResponseWriter, r *http.Request) {
	note := entity.Note{OwnerID: uidFromContext(r.Context())}
	// A note is created owned by whoever asked, so what this excludes is a caller with no uid,
	// which requireAuth has already refused. Asking anyway keeps creating a note in the same table
	// as every other action rather than being the one rule nothing writes down.
	if !requirePermission(w, r, note.Permission(entity.ActionCreate)) {
		return
	}
	if !decodeNoteRequest(w, r, &note) {
		return
	}

	created, err := s.notes.Create(r.Context(), note)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.writeNote(w, r, http.StatusCreated, created)
}

// UpdateNote replaces a note's client-settable fields. Because the request is applied to the
// stored note, the id, owner and createdAt carry over untouched.
func (s *Service) UpdateNote(w http.ResponseWriter, r *http.Request) {
	note, ok := s.requireNotePermission(w, r, entity.ActionUpdate)
	if !ok {
		return
	}
	if !decodeNoteRequest(w, r, &note) {
		return
	}

	updated, err := s.notes.Update(r.Context(), note)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "note not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.writeNote(w, r, http.StatusOK, updated)
}

// DeleteNote removes a note. Only the owner may delete it.
func (s *Service) DeleteNote(w http.ResponseWriter, r *http.Request) {
	note, ok := s.requireNotePermission(w, r, entity.ActionDelete)
	if !ok {
		return
	}

	if err := s.notes.Delete(r.Context(), note.ID); err != nil && !errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
