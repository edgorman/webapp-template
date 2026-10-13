package service

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/edgorman/webapp-template/services/backend/internal/entity"
	appv1 "github.com/edgorman/webapp-template/services/backend/internal/gen/app/v1"
)

func noteRequestBody(t *testing.T, body *appv1.NoteRequest) *bytes.Reader {
	t.Helper()

	encoded, err := protojson.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return bytes.NewReader(encoded)
}

// notePathRequest addresses a note the way its route does, by id.
func notePathRequest(method, id string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, "/notes/"+id, body)
	req.SetPathValue("id", id)
	return req
}

// wireNote and wireNotePage are what a response body actually carries, spelled out here rather
// than decoded into appv1.Note: reusing the production type would hide the wire, since both sides
// would move together and a field leaving the body would assert nothing. Declared separately, a
// rename fails here.
type wireNote struct {
	ID             string `json:"id"`
	AuthorUsername string `json:"authorUsername"`
	Title          string `json:"title"`
	Body           string `json:"body"`
	Visibility     string `json:"visibility"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

type wireNotePage struct {
	Notes   []wireNote `json:"notes"`
	HasMore bool       `json:"hasMore"`
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()

	var got T
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return got
}

func idsOf(page wireNotePage) []string {
	ids := make([]string, 0, len(page.Notes))
	for _, note := range page.Notes {
		ids = append(ids, note.ID)
	}
	return ids
}

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if got := rec.Result().StatusCode; got != want {
		t.Fatalf("status = %d, want %d (body %s)", got, want, rec.Body.String())
	}
}

// seededNotes is one public and one private note per owner, a minute apart, oldest first.
func seededNotes() *fakeNoteRepository {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	notes := newFakeNoteRepository()
	for i, note := range []entity.Note{
		{ID: "annpublic", OwnerID: "ann", Visibility: entity.VisibilityPublic},
		{ID: "annprivate", OwnerID: "ann", Visibility: entity.VisibilityPrivate},
		{ID: "bobpublic", OwnerID: "bob", Visibility: entity.VisibilityPublic},
		{ID: "bobprivate", OwnerID: "bob", Visibility: entity.VisibilityPrivate},
	} {
		note.CreatedAt = start.Add(time.Duration(i) * time.Minute)
		note.UpdatedAt = note.CreatedAt
		notes.seed(note)
	}
	return notes
}

func TestListNotes_OnlyReadableNotesNewestFirst(t *testing.T) {
	s := newTestService(seededNotes(), nil)

	for _, tt := range []struct {
		uid  string
		want string
	}{
		{"", "bobpublic annpublic"},
		{"ann", "bobpublic annprivate annpublic"},
		{"bob", "bobprivate bobpublic annpublic"},
	} {
		rec := httptest.NewRecorder()
		s.ListNotes(rec, withUID(httptest.NewRequest(http.MethodGet, "/notes", nil), tt.uid))
		assertStatus(t, rec, http.StatusOK)

		if got := strings.Join(idsOf(decode[wireNotePage](t, rec)), " "); got != tt.want {
			t.Errorf("uid %q sees %q, want %q", tt.uid, got, tt.want)
		}
	}
}

func TestListNotes_Paginates(t *testing.T) {
	s := newTestService(seededNotes(), nil)

	rec := httptest.NewRecorder()
	s.ListNotes(rec, withUID(httptest.NewRequest(http.MethodGet, "/notes?limit=1", nil), ""))
	first := decode[wireNotePage](t, rec)
	if strings.Join(idsOf(first), " ") != "bobpublic" || !first.HasMore {
		t.Fatalf("first page = %v (hasMore %v), want [bobpublic] with more", idsOf(first), first.HasMore)
	}

	rec = httptest.NewRecorder()
	next := "/notes?limit=1&startAfter=" + first.Notes[0].CreatedAt
	s.ListNotes(rec, withUID(httptest.NewRequest(http.MethodGet, next, nil), ""))
	second := decode[wireNotePage](t, rec)
	if strings.Join(idsOf(second), " ") != "annpublic" || second.HasMore {
		t.Fatalf("second page = %v (hasMore %v), want [annpublic] and no more", idsOf(second), second.HasMore)
	}
}

// An empty page is an empty array with hasMore false on the wire, not null and not absent.
func TestListNotes_EmptyWireBody(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestService(nil, nil).ListNotes(rec, httptest.NewRequest(http.MethodGet, "/notes", nil))

	if got := strings.TrimSpace(rec.Body.String()); got != `{"notes":[],"hasMore":false}` {
		t.Errorf("body = %s", got)
	}
}

func TestListNotes_RejectsMalformedParams(t *testing.T) {
	s := newTestService(nil, nil)
	for _, query := range []string{"limit=0", "limit=x", "startAfter=yesterday"} {
		rec := httptest.NewRecorder()
		s.ListNotes(rec, httptest.NewRequest(http.MethodGet, "/notes?"+query, nil))
		assertStatus(t, rec, http.StatusBadRequest)
		decodeAPIError(t, rec)
	}
}

// Authors are resolved per distinct owner, not per note, so one author's feed costs one lookup.
func TestListNotes_ResolvesEachAuthorOnce(t *testing.T) {
	notes := newFakeNoteRepository()
	for _, id := range []string{"n1", "n2", "n3"} {
		notes.seed(entity.Note{ID: id, OwnerID: "owner", Visibility: entity.VisibilityPublic})
	}
	users := newFakeUserRepository()
	users.seed(entity.User{ID: "owner", Username: "sly-dancing-monkey"})

	rec := httptest.NewRecorder()
	newTestService(notes, users).ListNotes(rec, httptest.NewRequest(http.MethodGet, "/notes", nil))

	for _, note := range decode[wireNotePage](t, rec).Notes {
		if note.AuthorUsername != "sly-dancing-monkey" {
			t.Errorf("note %s carries author %q, want the owner's username", note.ID, note.AuthorUsername)
		}
	}
	if users.gets != 1 {
		t.Errorf("user lookups = %d, want 1 for three notes by one author", users.gets)
	}
}

func TestGetNote_WireBody(t *testing.T) {
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	notes := newFakeNoteRepository()
	notes.seed(entity.Note{
		ID: "n1", OwnerID: "owner", Title: "Hello", Body: "World",
		Visibility: entity.VisibilityPublic, CreatedAt: at, UpdatedAt: at,
	})
	users := newFakeUserRepository()
	users.seed(entity.User{ID: "owner", Username: "sly-dancing-monkey"})

	rec := httptest.NewRecorder()
	newTestService(notes, users).GetNote(rec, notePathRequest(http.MethodGet, "n1", nil))

	want := `{"id":"n1","authorUsername":"sly-dancing-monkey","title":"Hello","body":"World","visibility":"public","createdAt":"2026-01-02T03:04:05Z","updatedAt":"2026-01-02T03:04:05Z"}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Errorf("body =\n%s\nwant\n%s", got, want)
	}
}

// A note the caller may not read answers exactly as a missing one does, for every action, so a
// refusal never confirms that a private note exists.
func TestNoteRoutes_UnreadableIsNotFound(t *testing.T) {
	s := newTestService(seededNotes(), nil)
	body := func() io.Reader {
		return noteRequestBody(t, &appv1.NoteRequest{Title: "t", Visibility: "public"})
	}

	for _, id := range []string{"annprivate", "missing"} {
		for name, run := range map[string]func(*httptest.ResponseRecorder){
			"get": func(rec *httptest.ResponseRecorder) {
				s.GetNote(rec, withUID(notePathRequest(http.MethodGet, id, nil), "bob"))
			},
			"update": func(rec *httptest.ResponseRecorder) {
				s.UpdateNote(rec, withUID(notePathRequest(http.MethodPut, id, body()), "bob"))
			},
			"delete": func(rec *httptest.ResponseRecorder) {
				s.DeleteNote(rec, withUID(notePathRequest(http.MethodDelete, id, nil), "bob"))
			},
		} {
			rec := httptest.NewRecorder()
			run(rec)
			if rec.Result().StatusCode != http.StatusNotFound {
				t.Errorf("%s %s = %d, want 404", name, id, rec.Result().StatusCode)
			}
		}
	}
}

// Writing a public note is still the owner's alone, and once the note is visible a refusal is an
// honest 403.
func TestNoteRoutes_WritesAreTheOwnersAlone(t *testing.T) {
	notes := seededNotes()
	s := newTestService(notes, nil)

	rec := httptest.NewRecorder()
	body := noteRequestBody(t, &appv1.NoteRequest{Title: "mine now", Visibility: "public"})
	s.UpdateNote(rec, withUID(notePathRequest(http.MethodPut, "annpublic", body), "bob"))
	assertStatus(t, rec, http.StatusForbidden)

	rec = httptest.NewRecorder()
	s.DeleteNote(rec, withUID(notePathRequest(http.MethodDelete, "annpublic", nil), "bob"))
	assertStatus(t, rec, http.StatusForbidden)

	if _, err := notes.Get(t.Context(), "annpublic"); err != nil {
		t.Errorf("note after refused writes: %v, want it untouched", err)
	}
}

func TestCreateNote_OwnedByTheCaller(t *testing.T) {
	notes := newFakeNoteRepository()
	s := newTestService(notes, nil)

	rec := httptest.NewRecorder()
	body := noteRequestBody(t, &appv1.NoteRequest{Title: "  Hello  ", Body: "World", Visibility: "private"})
	s.CreateNote(rec, withUID(httptest.NewRequest(http.MethodPost, "/notes", body), "ann"))
	assertStatus(t, rec, http.StatusCreated)

	got := decode[wireNote](t, rec)
	stored, err := notes.Get(t.Context(), got.ID)
	if err != nil {
		t.Fatalf("stored note: %v", err)
	}
	if stored.OwnerID != "ann" || stored.Title != "Hello" || stored.Visibility != entity.VisibilityPrivate {
		t.Errorf("stored = %+v, want ann's private note titled Hello", stored)
	}
}

func TestCreateNote_RejectsInvalidFields(t *testing.T) {
	s := newTestService(nil, nil)
	for _, req := range []*appv1.NoteRequest{
		{Title: "t", Visibility: "everyone"},
		{Title: strings.Repeat("a", entity.MaxTitleLength+1), Visibility: "public"},
	} {
		rec := httptest.NewRecorder()
		s.CreateNote(rec, withUID(httptest.NewRequest(http.MethodPost, "/notes", noteRequestBody(t, req)), "ann"))
		assertStatus(t, rec, http.StatusBadRequest)
		decodeAPIError(t, rec)
	}
}

// The request is applied to the stored note, so the id, owner and createdAt survive an edit.
func TestUpdateNote_KeepsServerSetFields(t *testing.T) {
	notes := seededNotes()
	before, _ := notes.Get(t.Context(), "annpublic")
	s := newTestService(notes, nil)

	rec := httptest.NewRecorder()
	body := noteRequestBody(t, &appv1.NoteRequest{Title: "Edited", Visibility: "private"})
	s.UpdateNote(rec, withUID(notePathRequest(http.MethodPut, "annpublic", body), "ann"))
	assertStatus(t, rec, http.StatusOK)

	after, _ := notes.Get(t.Context(), "annpublic")
	if after.Title != "Edited" || after.Visibility != entity.VisibilityPrivate {
		t.Errorf("stored = %+v, want the edit applied", after)
	}
	if after.OwnerID != "ann" || !after.CreatedAt.Equal(before.CreatedAt) {
		t.Errorf("stored = %+v, want the owner and createdAt carried over", after)
	}
}

func TestDeleteNote_Owner(t *testing.T) {
	notes := seededNotes()
	s := newTestService(notes, nil)

	rec := httptest.NewRecorder()
	s.DeleteNote(rec, withUID(notePathRequest(http.MethodDelete, "annprivate", nil), "ann"))
	assertStatus(t, rec, http.StatusNoContent)

	rec = httptest.NewRecorder()
	s.GetNote(rec, withUID(notePathRequest(http.MethodGet, "annprivate", nil), "ann"))
	assertStatus(t, rec, http.StatusNotFound)
}
