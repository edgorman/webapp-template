// Package firestore implements the repository interfaces against Cloud Firestore.
package firestore

import (
	"context"
	"regexp"
	"time"

	fs "cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/edgorman/webapp-template/services/backend/internal/entity"
	"github.com/edgorman/webapp-template/services/backend/internal/repository"
)

// noteDocument is the stored shape of a note. It exists so Firestore's field names live here
// rather than as a second set of tags on entity.Note. The id is the document key, so it is not
// stored in the body.
type noteDocument struct {
	OwnerID    string    `firestore:"ownerId"`
	Title      string    `firestore:"title"`
	Body       string    `firestore:"body"`
	Visibility string    `firestore:"visibility"`
	CreatedAt  time.Time `firestore:"createdAt"`
	UpdatedAt  time.Time `firestore:"updatedAt"`
}

func noteToDocument(note entity.Note) noteDocument {
	return noteDocument{
		OwnerID:    note.OwnerID,
		Title:      note.Title,
		Body:       note.Body,
		Visibility: string(note.Visibility),
		CreatedAt:  note.CreatedAt,
		UpdatedAt:  note.UpdatedAt,
	}
}

// toEntity rebuilds a note from its stored fields; id is the document key.
func (d noteDocument) toEntity(id string) entity.Note {
	return entity.Note{
		ID:         id,
		OwnerID:    d.OwnerID,
		Title:      d.Title,
		Body:       d.Body,
		Visibility: entity.Visibility(d.Visibility),
		CreatedAt:  d.CreatedAt,
		UpdatedAt:  d.UpdatedAt,
	}
}

func documentToNote(doc *fs.DocumentSnapshot) (entity.Note, error) {
	var stored noteDocument
	if err := doc.DataTo(&stored); err != nil {
		return entity.Note{}, err
	}
	return stored.toEntity(doc.Ref.ID), nil
}

// noteIDPattern admits the ids Create assigns (Firestore's 20-character auto ids) and nothing that
// could address anything but a single document: no slash, no "." or "..", no "__reserved__" form.
// An id arrives in a URL, so it is checked before Firestore is asked to parse it as a path.
var noteIDPattern = regexp.MustCompile(`^[A-Za-z0-9]{1,64}$`)

var _ repository.NoteRepository = (*NoteRepository)(nil)

type NoteRepository struct {
	client *fs.Client
	notes  *fs.CollectionRef
}

// NewNoteRepository returns a repository.NoteRepository backed by the "notes" collection.
func NewNoteRepository(client *fs.Client) *NoteRepository {
	return &NoteRepository{client: client, notes: client.Collection("notes")}
}

func (r *NoteRepository) Get(ctx context.Context, id string) (entity.Note, error) {
	if !noteIDPattern.MatchString(id) {
		return entity.Note{}, repository.ErrNotFound
	}

	doc, err := r.notes.Doc(id).Get(ctx)
	if status.Code(err) == codes.NotFound {
		return entity.Note{}, repository.ErrNotFound
	}
	if err != nil {
		return entity.Note{}, err
	}
	return documentToNote(doc)
}

// maxNoteListPageSize is the largest page List ever returns, whatever a caller asks for.
const maxNoteListPageSize = 50

// List runs CanBeReadBy's predicate as a Firestore OR query (public, or the caller's own), so a
// caller never fetches a note they may not read, and asks for one more than the page holds: its
// presence is what hasMore reports. Each OR branch needs a composite index with createdAt, which
// infrastructure/env/firestore.tf declares.
//
// Every note is still rechecked with CanBeReadBy, so the query and the entity cannot drift apart
// unnoticed: a mismatch would cost a short page rather than leak a note.
func (r *NoteRepository) List(ctx context.Context, uid string, params repository.ListParams) ([]entity.Note, bool, error) {
	limit := params.Limit
	if limit <= 0 || limit > maxNoteListPageSize {
		limit = maxNoteListPageSize
	}

	public := fs.PropertyFilter{Path: "visibility", Operator: "==", Value: string(entity.VisibilityPublic)}
	query := r.notes.WhereEntity(public)
	if uid != "" {
		query = r.notes.WhereEntity(fs.OrFilter{Filters: []fs.EntityFilter{
			public,
			fs.PropertyFilter{Path: "ownerId", Operator: "==", Value: uid},
		}})
	}
	query = query.OrderBy("createdAt", fs.Desc).Limit(limit + 1)
	if !params.StartAfter.IsZero() {
		query = query.StartAfter(params.StartAfter)
	}

	docs, err := query.Documents(ctx).GetAll()
	if err != nil {
		return nil, false, err
	}

	hasMore := len(docs) > limit
	if hasMore {
		docs = docs[:limit]
	}
	notes := make([]entity.Note, 0, len(docs))
	for _, doc := range docs {
		note, err := documentToNote(doc)
		if err != nil {
			return nil, false, err
		}
		if note.CanBeReadBy(uid) {
			notes = append(notes, note)
		}
	}
	return notes, hasMore, nil
}

func (r *NoteRepository) Create(ctx context.Context, note entity.Note) (entity.Note, error) {
	if err := note.Validate(); err != nil {
		return entity.Note{}, err
	}

	now := time.Now().UTC()
	note.CreatedAt = now
	note.UpdatedAt = now

	doc := r.notes.NewDoc()
	if _, err := doc.Create(ctx, noteToDocument(note)); err != nil {
		return entity.Note{}, err
	}
	note.ID = doc.ID
	return note, nil
}

// Update requires the document to still exist, so a note deleted since its caller read it is not
// silently recreated.
func (r *NoteRepository) Update(ctx context.Context, note entity.Note) (entity.Note, error) {
	if err := note.Validate(); err != nil {
		return entity.Note{}, err
	}
	if !noteIDPattern.MatchString(note.ID) {
		return entity.Note{}, repository.ErrNotFound
	}

	note.UpdatedAt = time.Now().UTC()
	doc := noteToDocument(note)
	_, err := r.notes.Doc(note.ID).Update(ctx, []fs.Update{
		{Path: "title", Value: doc.Title},
		{Path: "body", Value: doc.Body},
		{Path: "visibility", Value: doc.Visibility},
		{Path: "updatedAt", Value: doc.UpdatedAt},
	})
	if status.Code(err) == codes.NotFound {
		return entity.Note{}, repository.ErrNotFound
	}
	if err != nil {
		return entity.Note{}, err
	}
	return note, nil
}

func (r *NoteRepository) Delete(ctx context.Context, id string) error {
	if !noteIDPattern.MatchString(id) {
		return repository.ErrNotFound
	}

	_, err := r.notes.Doc(id).Delete(ctx, fs.Exists)
	if status.Code(err) == codes.NotFound {
		return repository.ErrNotFound
	}
	return err
}

func (r *NoteRepository) DeleteOwned(ctx context.Context, ownerID string) error {
	docs, err := r.notes.Where("ownerId", "==", ownerID).Select().Documents(ctx).GetAll()
	if err != nil {
		return err
	}
	refs := make([]*fs.DocumentRef, 0, len(docs))
	for _, doc := range docs {
		refs = append(refs, doc.Ref)
	}
	return deleteAll(ctx, r.client, refs)
}

// deleteAll deletes refs through a BulkWriter and reports the first delete that failed. A
// BulkWriter's End does not return errors itself, so each job is asked for its result.
func deleteAll(ctx context.Context, client *fs.Client, refs []*fs.DocumentRef) error {
	if len(refs) == 0 {
		return nil
	}
	batch := client.BulkWriter(ctx)
	jobs := make([]*fs.BulkWriterJob, 0, len(refs))
	for _, ref := range refs {
		job, err := batch.Delete(ref)
		if err != nil {
			batch.End()
			return err
		}
		jobs = append(jobs, job)
	}
	batch.End()

	for _, job := range jobs {
		if _, err := job.Results(); err != nil {
			return err
		}
	}
	return nil
}
