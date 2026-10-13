package entity

import (
	"encoding/json"
	"testing"
	"time"
)

// entity.User's json tags are the field names debug output and logs carry, so a rename shows here.
func TestJSONFieldNamesMatchFrontendContract(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	user, err := json.Marshal(User{ID: "u1", Username: "sly-dancing-monkey", Bio: "hi", CreatedAt: at, UpdatedAt: at})
	if err != nil {
		t.Fatalf("marshal user: %v", err)
	}
	wantUser := `{"id":"u1","username":"sly-dancing-monkey","bio":"hi","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}`
	if string(user) != wantUser {
		t.Errorf("user JSON =\n %s\nwant\n %s", user, wantUser)
	}
}
