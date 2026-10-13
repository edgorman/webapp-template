package firestore

import (
	"context"
	"errors"
	"testing"

	"github.com/edgorman/webapp-template/services/backend/internal/entity"
	"github.com/edgorman/webapp-template/services/backend/internal/repository"
)

// Each repository is built with a nil collection, which panics on any Firestore call. Returning a
// clean validation error instead therefore proves the write was refused before the datastore was
// touched, rather than merely that it was refused.
func TestRepositoriesRejectInvalidEntitiesBeforeWriting(t *testing.T) {
	ctx := context.Background()

	t.Run("note create", func(t *testing.T) {
		_, err := (&NoteRepository{}).Create(ctx, entity.Note{Visibility: entity.VisibilityPublic})
		assertValidationError(t, err)
	})

	t.Run("note update", func(t *testing.T) {
		_, err := (&NoteRepository{}).Update(ctx, entity.Note{ID: "n1", Visibility: entity.VisibilityPublic})
		assertValidationError(t, err)
	})

	t.Run("note with a bad visibility", func(t *testing.T) {
		_, err := (&NoteRepository{}).Create(ctx, entity.Note{OwnerID: "owner", Visibility: "everyone"})
		assertValidationError(t, err)
	})

	t.Run("user put", func(t *testing.T) {
		_, err := (&UserRepository{}).Put(ctx, entity.User{Username: "sly-dancing-monkey"})
		assertValidationError(t, err)
	})

	t.Run("user with no username", func(t *testing.T) {
		_, err := (&UserRepository{}).Put(ctx, entity.User{ID: "u1"})
		assertValidationError(t, err)
	})

	t.Run("user with a malformed username", func(t *testing.T) {
		_, err := (&UserRepository{}).Put(ctx, entity.User{ID: "u1", Username: "sly dancing monkey"})
		assertValidationError(t, err)
	})

	t.Run("user with a reserved username", func(t *testing.T) {
		_, err := (&UserRepository{}).Put(ctx, entity.User{ID: "u1", Username: "me"})
		assertValidationError(t, err)
	})
}

func assertValidationError(t *testing.T, err error) {
	t.Helper()

	var invalid entity.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("err = %v, want an entity.ValidationError", err)
	}
	if invalid.Field == "" {
		t.Error("ValidationError does not name a field")
	}
}

// A malformed id is a miss rather than a path Firestore is asked to parse, which it would answer
// by panicking rather than erroring.
func TestNoteRepositoryRefusesUnaddressableIDs(t *testing.T) {
	ctx := context.Background()
	notes := &NoteRepository{}

	for _, id := range []string{"", "a/b", ".."} {
		if _, err := notes.Get(ctx, id); !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("Get(%q) = %v, want ErrNotFound", id, err)
		}
		if err := notes.Delete(ctx, id); !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("Delete(%q) = %v, want ErrNotFound", id, err)
		}
		valid := entity.Note{ID: id, OwnerID: "owner", Visibility: entity.VisibilityPublic}
		if _, err := notes.Update(ctx, valid); !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("Update(%q) = %v, want ErrNotFound", id, err)
		}
	}
}
