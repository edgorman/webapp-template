package repository

import (
	"context"
	"time"

	"github.com/edgorman/webapp-template/services/backend/internal/entity"
)

// ListParams bounds one page of NoteRepository.List.
type ListParams struct {
	// Limit caps how many notes a page holds. A repository is free to treat a non-positive or
	// out-of-range value as its own default rather than erroring, since this is a page size, not a
	// caller-facing validation.
	Limit int
	// StartAfter, when non-zero, excludes every note at or after this createdAt - continuing a feed
	// a caller has already seen a page of. A caller sets it to the createdAt of the last note its
	// previous page held.
	StartAfter time.Time
}

// NoteRepository persists notes. Get is unfiltered: callers check read access themselves via
// entity.Note.CanBeReadBy.
type NoteRepository interface {
	// Get returns ErrNotFound if no note holds id, including an id that could not be a key at all.
	Get(ctx context.Context, id string) (entity.Note, error)
	// List returns one page of the notes uid may read, newest first, applying the same predicate as
	// entity.Note.CanBeReadBy, and reports whether a further page follows.
	List(ctx context.Context, uid string, params ListParams) (notes []entity.Note, hasMore bool, err error)
	// Create writes a new note under an id the repository assigns, stamping the creation/update
	// timestamps. It rejects a note that fails entity.Note.Validate without writing anything.
	Create(ctx context.Context, note entity.Note) (entity.Note, error)
	// Update overwrites the note at note.ID and refreshes UpdatedAt, carrying CreatedAt over from
	// note. It rejects a note that fails entity.Note.Validate without writing anything, and returns
	// ErrNotFound if the note has been deleted since it was read.
	Update(ctx context.Context, note entity.Note) (entity.Note, error)
	// Delete removes the note at id, returning ErrNotFound if there is none.
	Delete(ctx context.Context, id string) error
	// DeleteOwned removes every note ownerID holds. It is for erasing an account.
	DeleteOwned(ctx context.Context, ownerID string) error
}
