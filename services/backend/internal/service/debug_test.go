package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/edgorman/webapp-template/services/backend/internal/entity"
)

func TestDebug(t *testing.T) {
	s := newTestService(nil, nil)

	rec := httptest.NewRecorder()
	s.Debug(rec, httptest.NewRequest(http.MethodGet, "/debug", nil))

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", ct, "application/json")
	}

	var body debugResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Status != "ok" {
		t.Errorf("Status = %q, want %q", body.Status, "ok")
	}
	if body.Environment != "test" {
		t.Errorf("Environment = %q, want %q", body.Environment, "test")
	}
	if body.Commit != "abc123" {
		t.Errorf("Commit = %q, want %q", body.Commit, "abc123")
	}
	if _, err := time.Parse(time.RFC3339, body.Timestamp); err != nil {
		t.Errorf("Timestamp = %q is not RFC3339: %v", body.Timestamp, err)
	}
}

// /health and /debug are the same handler, and neither requires a credential.
func TestHandler_DebugRoutesAreUnauthenticated(t *testing.T) {
	s := newTestService(nil, nil)

	for _, path := range []string{"/health", "/debug"} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

			if rec.Result().StatusCode != http.StatusOK {
				t.Errorf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
			}
		})
	}
}

// Every write sits behind requireAuth.
func TestHandler_WriteRoutesRequireAuth(t *testing.T) {
	s := newTestService(nil, nil)

	for _, tt := range []struct{ method, path string }{
		{http.MethodPost, "/notes"},
		{http.MethodPut, "/notes/n1"},
		{http.MethodDelete, "/notes/n1"},
		{http.MethodGet, "/users/me"},
		{http.MethodPut, "/users/me"},
		{http.MethodDelete, "/users/me"},
	} {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			s.Handler().ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))

			if rec.Result().StatusCode != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", rec.Result().StatusCode, http.StatusUnauthorized)
			}
		})
	}
}

// Reads admit anonymous callers - they 401 only for a credential that is present but invalid,
// never merely absent - and see only what is public.
func TestHandler_ReadRoutesAdmitAnonymousCallers(t *testing.T) {
	notes := newFakeNoteRepository()
	notes.seed(
		entity.Note{ID: "public", OwnerID: "owner", Visibility: entity.VisibilityPublic},
		entity.Note{ID: "private", OwnerID: "owner", Visibility: entity.VisibilityPrivate},
	)
	users := newFakeUserRepository()
	users.seed(entity.User{ID: "owner", Username: "quiet-reading-otter"})
	s := newTestService(notes, users)

	for _, tt := range []struct {
		path string
		want int
	}{
		{"/notes", http.StatusOK},
		{"/notes/public", http.StatusOK},
		// Not a 401, since no credential was required, and not a 403, which would confirm the note
		// exists: the same 404 a signed-in stranger gets.
		{"/notes/private", http.StatusNotFound},
		{"/users/quiet-reading-otter", http.StatusOK},
	} {
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))

		if rec.Result().StatusCode != tt.want {
			t.Errorf("GET %s = %d, want %d", tt.path, rec.Result().StatusCode, tt.want)
		}
	}

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/notes", nil))
	if got := idsOf(decode[wireNotePage](t, rec)); len(got) != 1 || got[0] != "public" {
		t.Errorf("anonymous GET /notes = %v, want only the public note", got)
	}
}
