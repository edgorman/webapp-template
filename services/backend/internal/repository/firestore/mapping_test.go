package firestore

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/edgorman/webapp-template/services/backend/internal/entity"
)

var (
	created = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	updated = time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
)

// A field added to entity.Note but not to the mapping would silently stop being persisted, so the
// round trip is asserted whole rather than field by field.
func TestNoteMappingRoundTrip(t *testing.T) {
	note := entity.Note{
		ID:         "abc123",
		OwnerID:    "owner",
		Title:      "Hello",
		Body:       "Body",
		Visibility: entity.VisibilityPrivate,
		CreatedAt:  created,
		UpdatedAt:  updated,
	}

	got := noteToDocument(note).toEntity(note.ID)

	if !reflect.DeepEqual(got, note) {
		t.Errorf("round trip = %+v, want %+v", got, note)
	}
}

// A note id arrives in a URL and becomes a document key, so anything that could address more than
// one document, or that Firestore refuses as a key, has to be refused before it is asked.
func TestNoteIDPattern(t *testing.T) {
	for _, id := range []string{"Ab3dEf6hIj9lMn2pQr5t", "a"} {
		if !noteIDPattern.MatchString(id) {
			t.Errorf("%q refused, want it admitted", id)
		}
	}
	for _, id := range []string{"", ".", "..", "a/b", "__x__", "has space", strings.Repeat("a", 65)} {
		if noteIDPattern.MatchString(id) {
			t.Errorf("%q admitted, want it refused", id)
		}
	}
}

func TestUserMappingRoundTrip(t *testing.T) {
	user := entity.User{
		ID:        "user-1",
		Username:  "sly-dancing-monkey",
		Bio:       "hello",
		CreatedAt: created,
		UpdatedAt: updated,
	}

	got := userToDocument(user).toEntity(user.ID)

	if !reflect.DeepEqual(got, user) {
		t.Errorf("round trip = %+v, want %+v", got, user)
	}
}

// An id is its document key, so storing it in the body too would duplicate it.
func TestUserDocumentExcludesID(t *testing.T) {
	if _, ok := reflect.TypeOf(userToDocument(entity.User{ID: "user-1"})).FieldByName("ID"); ok {
		t.Error("user document type has an ID field, which would duplicate the document key")
	}
}

func TestNoteDocumentExcludesID(t *testing.T) {
	if _, ok := reflect.TypeOf(noteToDocument(entity.Note{ID: "n1"})).FieldByName("ID"); ok {
		t.Error("note document type has an ID field, which would duplicate the document key")
	}
}
