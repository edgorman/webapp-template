package firestore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	fs "cloud.google.com/go/firestore"

	"github.com/edgorman/webapp-template/services/backend/internal/entity"
	"github.com/edgorman/webapp-template/services/backend/internal/repository"
)

// These tests run the adapters against the Firestore emulator, so the queries, transactions and
// collection-group reads are exercised by Firestore itself rather than by a fake's idea of it. They
// skip when FIRESTORE_EMULATOR_HOST is unset, which keeps `go test ./...` passing without one; CI
// starts one (see the `test` job in .github/workflows/pull-request.yaml). The emulator does not
// enforce composite indexes, so a missing index is still only caught in staging.

// newClient connects to the emulator under a project of the test's own: the emulator keeps each
// project's data apart, so no test sees another's documents and none needs cleaning up after.
func newClient(t *testing.T) *fs.Client {
	t.Helper()
	if os.Getenv("FIRESTORE_EMULATOR_HOST") == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST is unset; see services/backend/README.md to run these")
	}
	client, err := fs.NewClient(context.Background(), fmt.Sprintf("test-%d", time.Now().UnixNano()))
	if err != nil {
		t.Fatalf("firestore client: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func createNote(t *testing.T, notes *NoteRepository, note entity.Note) entity.Note {
	t.Helper()
	if note.Visibility == "" {
		note.Visibility = entity.VisibilityPublic
	}
	created, err := notes.Create(context.Background(), note)
	if err != nil {
		t.Fatalf("Create(%s): %v", note.Title, err)
	}
	return created
}

func titlesOf(notes []entity.Note) []string {
	titles := make([]string, 0, len(notes))
	for _, note := range notes {
		titles = append(titles, note.Title)
	}
	return titles
}

func TestEmulatorNote_CreateGetUpdateDelete(t *testing.T) {
	ctx := context.Background()
	notes := NewNoteRepository(newClient(t))

	created := createNote(t, notes, entity.Note{OwnerID: "ann", Title: "Hello", Body: "Body"})
	if created.ID == "" || created.CreatedAt.IsZero() {
		t.Fatalf("Create = %+v, want an id and a creation time", created)
	}
	got, err := notes.Get(ctx, created.ID)
	if err != nil || got.Title != "Hello" || got.OwnerID != "ann" {
		t.Fatalf("Get = %+v, %v; want the created note", got, err)
	}

	got.Title = "Edited"
	if _, err := notes.Update(ctx, got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got, _ := notes.Get(ctx, created.ID); got.Title != "Edited" || !got.CreatedAt.Equal(created.CreatedAt.Truncate(time.Microsecond)) {
		t.Errorf("stored = %+v, want the new title and the original creation time", got)
	}

	if err := notes.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := notes.Get(ctx, created.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Get after Delete = %v, want ErrNotFound", err)
	}
	if err := notes.Delete(ctx, created.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("repeated Delete = %v, want ErrNotFound", err)
	}
	// Updating a note deleted since it was read must not recreate it.
	if _, err := notes.Update(ctx, got); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Update after Delete = %v, want ErrNotFound", err)
	}
}

func TestEmulatorNote_ListReadableAndPaged(t *testing.T) {
	ctx := context.Background()
	notes := NewNoteRepository(newClient(t))
	createNote(t, notes, entity.Note{OwnerID: "ann", Title: "one"})
	createNote(t, notes, entity.Note{OwnerID: "ann", Title: "two", Visibility: entity.VisibilityPrivate})
	createNote(t, notes, entity.Note{OwnerID: "bob", Title: "three", Visibility: entity.VisibilityPrivate})
	createNote(t, notes, entity.Note{OwnerID: "bob", Title: "four"})

	anonymous, _, err := notes.List(ctx, "", repository.ListParams{})
	if err != nil || !slices.Equal(titlesOf(anonymous), []string{"four", "one"}) {
		t.Errorf("anonymous List = %v, %v; want [four one]", titlesOf(anonymous), err)
	}
	ann, _, err := notes.List(ctx, "ann", repository.ListParams{})
	if err != nil || !slices.Equal(titlesOf(ann), []string{"four", "two", "one"}) {
		t.Errorf("ann's List = %v, %v; want [four two one]", titlesOf(ann), err)
	}

	first, hasMore, err := notes.List(ctx, "ann", repository.ListParams{Limit: 2})
	if err != nil || !slices.Equal(titlesOf(first), []string{"four", "two"}) || !hasMore {
		t.Fatalf("first page = %v (hasMore %v, err %v), want [four two] with more", titlesOf(first), hasMore, err)
	}
	second, hasMore, err := notes.List(ctx, "ann", repository.ListParams{Limit: 2, StartAfter: first[1].CreatedAt})
	if err != nil || !slices.Equal(titlesOf(second), []string{"one"}) || hasMore {
		t.Fatalf("second page = %v (hasMore %v, err %v), want [one] and no more", titlesOf(second), hasMore, err)
	}
}

func TestEmulatorNote_DeleteOwned(t *testing.T) {
	ctx := context.Background()
	notes := NewNoteRepository(newClient(t))
	createNote(t, notes, entity.Note{OwnerID: "ann", Title: "one"})
	createNote(t, notes, entity.Note{OwnerID: "ann", Title: "two", Visibility: entity.VisibilityPrivate})
	createNote(t, notes, entity.Note{OwnerID: "bob", Title: "three"})

	if err := notes.DeleteOwned(ctx, "ann"); err != nil {
		t.Fatalf("DeleteOwned: %v", err)
	}
	left, _, err := notes.List(ctx, "ann", repository.ListParams{})
	if err != nil || !slices.Equal(titlesOf(left), []string{"three"}) {
		t.Errorf("List after DeleteOwned = %v, %v; want [three]", titlesOf(left), err)
	}
}

func TestEmulatorUser(t *testing.T) {
	ctx := context.Background()
	users := NewUserRepository(newClient(t))

	if _, err := users.Put(ctx, entity.User{ID: "ann", Username: "Ann"}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := users.Put(ctx, entity.User{ID: "bob", Username: "ann"}); !errors.Is(err, repository.ErrUsernameTaken) {
		t.Errorf("Put with a held username = %v, want ErrUsernameTaken", err)
	}

	got, err := users.GetByUsername(ctx, "ANN")
	if err != nil || got.ID != "ann" {
		t.Fatalf("GetByUsername = %+v, %v; want ann", got, err)
	}

	// A rename releases the old name for anybody to claim.
	if _, err := users.Put(ctx, entity.User{ID: "ann", Username: "annie", Bio: "hi"}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := users.GetByUsername(ctx, "ann"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("GetByUsername(old name) = %v, want ErrNotFound", err)
	}
	if _, err := users.Put(ctx, entity.User{ID: "bob", Username: "ann"}); err != nil {
		t.Errorf("Put claiming a released name = %v, want nil", err)
	}
	if got, err := users.Get(ctx, "ann"); err != nil || got.Username != "annie" || got.Bio != "hi" || got.CreatedAt.IsZero() {
		t.Errorf("Get = %+v, %v; want annie with a bio and a creation time", got, err)
	}

	if err := users.Delete(ctx, "ann"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := users.Get(ctx, "ann"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Get after Delete = %v, want ErrNotFound", err)
	}
	if _, err := users.GetByUsername(ctx, "annie"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("GetByUsername after Delete = %v, want ErrNotFound", err)
	}
	if err := users.Delete(ctx, "ann"); err != nil {
		t.Errorf("repeated Delete = %v, want nil", err)
	}
}
