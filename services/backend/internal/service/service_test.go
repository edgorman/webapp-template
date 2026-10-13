package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/edgorman/webapp-template/services/backend/internal/entity"
	"github.com/edgorman/webapp-template/services/backend/internal/repository"
)

// fakeNoteRepository is an in-memory repository.NoteRepository.
type fakeNoteRepository struct {
	notes  map[string]entity.Note
	nextID int
	// deleteOwnedErr fails the next DeleteOwned, and is cleared by it, so a test can cut an
	// erasure off partway.
	deleteOwnedErr error
}

func newFakeNoteRepository() *fakeNoteRepository {
	return &fakeNoteRepository{notes: make(map[string]entity.Note)}
}

func (r *fakeNoteRepository) seed(notes ...entity.Note) {
	for _, note := range notes {
		r.notes[note.ID] = note
	}
}

func (r *fakeNoteRepository) Get(_ context.Context, id string) (entity.Note, error) {
	note, ok := r.notes[id]
	if !ok {
		return entity.Note{}, repository.ErrNotFound
	}
	return note, nil
}

// List mirrors the Firestore repository's contract - filter by readability, sort newest first, cut
// at the StartAfter cursor, cap at Limit, report whether anything was cut.
func (r *fakeNoteRepository) List(_ context.Context, uid string, params repository.ListParams) ([]entity.Note, bool, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = len(r.notes) + 1
	}

	visible := make([]entity.Note, 0, len(r.notes))
	for _, note := range r.notes {
		if !note.CanBeReadBy(uid) {
			continue
		}
		if !params.StartAfter.IsZero() && !note.CreatedAt.Before(params.StartAfter) {
			continue
		}
		visible = append(visible, note)
	}
	slices.SortFunc(visible, func(a, b entity.Note) int { return b.CreatedAt.Compare(a.CreatedAt) })

	hasMore := len(visible) > limit
	if hasMore {
		visible = visible[:limit]
	}
	return visible, hasMore, nil
}

func (r *fakeNoteRepository) Create(_ context.Context, note entity.Note) (entity.Note, error) {
	if err := note.Validate(); err != nil {
		return entity.Note{}, err
	}
	r.nextID++
	note.ID = fmt.Sprintf("note%d", r.nextID)
	now := time.Now().UTC()
	note.CreatedAt = now
	note.UpdatedAt = now
	r.seed(note)
	return note, nil
}

func (r *fakeNoteRepository) Update(_ context.Context, note entity.Note) (entity.Note, error) {
	if err := note.Validate(); err != nil {
		return entity.Note{}, err
	}
	if _, ok := r.notes[note.ID]; !ok {
		return entity.Note{}, repository.ErrNotFound
	}
	note.UpdatedAt = time.Now().UTC()
	r.seed(note)
	return note, nil
}

func (r *fakeNoteRepository) Delete(_ context.Context, id string) error {
	if _, ok := r.notes[id]; !ok {
		return repository.ErrNotFound
	}
	delete(r.notes, id)
	return nil
}

func (r *fakeNoteRepository) DeleteOwned(_ context.Context, ownerID string) error {
	if err := r.deleteOwnedErr; err != nil {
		r.deleteOwnedErr = nil
		return err
	}
	for id, note := range r.notes {
		if note.OwnerID == ownerID {
			delete(r.notes, id)
		}
	}
	return nil
}

// fakeUserRepository is an in-memory repository.UserRepository. usernames mirrors the reservation
// collection the Firestore implementation keeps, so uniqueness is enforced here too rather than
// assumed. gets counts lookups so a test can assert a handler did not reach the datastore.
type fakeUserRepository struct {
	users     map[string]entity.User
	usernames map[string]string
	gets      int
	// beforePut lets a test fail a write the in-memory state would otherwise allow, which is the
	// only way to provoke a username collision when the name being written is generated at random.
	beforePut func(entity.User) error
	// getErr fails a lookup by id the in-memory state would otherwise answer, for a test asserting
	// that a handler which needs the caller's own profile does not carry on without it.
	getErr error
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{
		users:     make(map[string]entity.User),
		usernames: make(map[string]string),
	}
}

// seed stores a profile and its reservation together, as a real write would.
func (r *fakeUserRepository) seed(user entity.User) {
	r.users[user.ID] = user
	r.usernames[user.UsernameKey()] = user.ID
}

func (r *fakeUserRepository) Get(_ context.Context, id string) (entity.User, error) {
	r.gets++
	if r.getErr != nil {
		return entity.User{}, r.getErr
	}
	user, ok := r.users[id]
	if !ok {
		return entity.User{}, repository.ErrNotFound
	}
	return user, nil
}

func (r *fakeUserRepository) GetByUsername(_ context.Context, username string) (entity.User, error) {
	r.gets++
	id, ok := r.usernames[strings.ToLower(username)]
	if !ok {
		return entity.User{}, repository.ErrNotFound
	}
	user, ok := r.users[id]
	if !ok {
		return entity.User{}, repository.ErrNotFound
	}
	return user, nil
}

func (r *fakeUserRepository) Put(_ context.Context, user entity.User) (entity.User, error) {
	user, err := user.Normalized()
	if err != nil {
		return entity.User{}, err
	}
	if r.beforePut != nil {
		if err := r.beforePut(user); err != nil {
			return entity.User{}, err
		}
	}
	key := user.UsernameKey()
	if owner, claimed := r.usernames[key]; claimed && owner != user.ID {
		return entity.User{}, repository.ErrUsernameTaken
	}

	now := time.Now().UTC()
	if previous, ok := r.users[user.ID]; ok {
		user.CreatedAt = previous.CreatedAt
		delete(r.usernames, previous.UsernameKey())
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	user.UpdatedAt = now

	r.users[user.ID] = user
	r.usernames[key] = user.ID
	return user, nil
}

func (r *fakeUserRepository) Delete(_ context.Context, id string) error {
	if user, ok := r.users[id]; ok {
		delete(r.usernames, user.UsernameKey())
	}
	delete(r.users, id)
	return nil
}

// fakeVerifier is an in-memory repository.TokenVerifier, standing in for the signed identity a
// provider would return. Only the uid is configurable, since that is the whole of what anything
// downstream decides with.
type fakeVerifier struct {
	uid string
	err error
}

func (f fakeVerifier) Verify(_ context.Context, _ string) (entity.Caller, error) {
	if f.err != nil {
		return entity.Caller{}, f.err
	}
	return entity.Caller{UID: f.uid, Email: "user@example.com", Name: "User", EmailVerified: true}, nil
}

// newTestService builds a Service over the given fakes, filling in whichever is nil.
func newTestService(notes repository.NoteRepository, users repository.UserRepository) *Service {
	if notes == nil {
		notes = newFakeNoteRepository()
	}
	if users == nil {
		users = newFakeUserRepository()
	}
	return New(
		Config{
			Environment: "test",
			Commit:      "abc123",
			// Quiet by default; a test asserting on what is logged points this at a buffer.
			Logger: slog.New(slog.DiscardHandler),
		},
		notes, users, fakeVerifier{uid: "caller"},
	)
}

func withUID(req *http.Request, uid string) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), callerContextKey, entity.Caller{UID: uid}))
}

// wireError is what a non-2xx response body actually carries, spelled out here rather than
// decoded into appv1.ErrorResponse - see wireNote (note_test.go) for why: reusing the production type would hide the wire, since both sides would move together
// and a field leaving the body would assert nothing. Declared separately, a rename fails here.
type wireError struct {
	Error string `json:"error"`
}

// decodeAPIError asserts the response carries a JSON error body rather than plain text.
func decodeAPIError(t *testing.T, rec *httptest.ResponseRecorder) wireError {
	t.Helper()

	if ct := rec.Result().Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
	var body wireError
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error == "" {
		t.Error("error body has an empty message")
	}
	return body
}
