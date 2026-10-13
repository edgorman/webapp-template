package entity

import (
	"errors"
	"strings"
	"testing"
)

func TestNote_SetVisibility(t *testing.T) {
	for _, tt := range []struct {
		input Visibility
		valid bool
	}{
		{VisibilityPublic, true},
		{VisibilityPrivate, true},
		{"everyone", false},
		{"", false},
		{"Public", false},
	} {
		t.Run(string(tt.input), func(t *testing.T) {
			note := Note{Visibility: VisibilityPublic}
			err := note.SetVisibility(tt.input)

			if tt.valid {
				if err != nil {
					t.Fatalf("SetVisibility(%q) = %v, want no error", tt.input, err)
				}
				if note.Visibility != tt.input {
					t.Errorf("Visibility = %q, want %q", note.Visibility, tt.input)
				}
				return
			}

			var invalid ValidationError
			if !errors.As(err, &invalid) {
				t.Fatalf("SetVisibility(%q) = %v, want a ValidationError", tt.input, err)
			}
			if note.Visibility != VisibilityPublic {
				t.Errorf("Visibility = %q, want the rejected value not to be applied", note.Visibility)
			}
		})
	}
}

// An empty title is legitimate - the frontend renders those as "(untitled)".
func TestNote_SetTitle(t *testing.T) {
	var note Note

	if err := note.SetTitle("  Hello  "); err != nil {
		t.Fatalf("SetTitle = %v, want no error", err)
	}
	if note.Title != "Hello" {
		t.Errorf("Title = %q, want %q", note.Title, "Hello")
	}
	if err := note.SetTitle(""); err != nil {
		t.Errorf("SetTitle(\"\") = %v, want no error", err)
	}
	if err := note.SetTitle(strings.Repeat("a", MaxTitleLength+1)); err == nil {
		t.Error("SetTitle over the limit = nil, want a ValidationError")
	}
}

// Whitespace is significant in a body, so it is not trimmed.
func TestNote_SetBody_PreservesWhitespace(t *testing.T) {
	var note Note

	if err := note.SetBody("\n  indented\n"); err != nil {
		t.Fatalf("SetBody = %v, want no error", err)
	}
	if note.Body != "\n  indented\n" {
		t.Errorf("Body = %q, want the whitespace preserved", note.Body)
	}
	if err := note.SetBody(strings.Repeat("a", MaxBodyLength+1)); err == nil {
		t.Error("SetBody over the limit = nil, want a ValidationError")
	}
}

func TestNote_CanBeReadBy(t *testing.T) {
	for _, tt := range []struct {
		name string
		note Note
		uid  string
		want bool
	}{
		{"public note, any caller", Note{OwnerID: "owner", Visibility: VisibilityPublic}, "someone", true},
		{"public note, anonymous caller", Note{OwnerID: "owner", Visibility: VisibilityPublic}, "", true},
		{"private note, owner", Note{OwnerID: "owner", Visibility: VisibilityPrivate}, "owner", true},
		{"private note, stranger", Note{OwnerID: "owner", Visibility: VisibilityPrivate}, "stranger", false},
		{"unowned private note, empty uid", Note{Visibility: VisibilityPrivate}, "", false},
		// A stored note carrying a visibility this build does not know reads as closed rather than
		// open, which is the way round to get an unrecognised value wrong.
		{"unrecognised visibility, stranger", Note{OwnerID: "owner", Visibility: "everyone"}, "stranger", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.note.CanBeReadBy(tt.uid); got != tt.want {
				t.Errorf("CanBeReadBy(%q) = %v, want %v", tt.uid, got, tt.want)
			}
		})
	}
}

// Writing a note is the owner's alone whatever its visibility - a public note is not one strangers
// may edit.
func TestNote_PermissionToWrite(t *testing.T) {
	note := Note{OwnerID: "owner", Visibility: VisibilityPublic}

	for _, action := range []Action{ActionCreate, ActionUpdate, ActionDelete} {
		if !note.Permission(action).Allows("owner") {
			t.Errorf("Permission(%q) refused the owner", action)
		}
		for _, uid := range []string{"stranger", ""} {
			if note.Permission(action).Allows(uid) {
				t.Errorf("Permission(%q).Allows(%q) = true, want only the owner", action, uid)
			}
		}
	}
}

// An unowned note must not be claimed by a caller whose uid is also empty.
func TestNote_IsOwnedBy_EmptyUIDNeverMatches(t *testing.T) {
	if (Note{}).IsOwnedBy("") {
		t.Error("IsOwnedBy(\"\") = true for an unowned note, want false")
	}
	if !(Note{OwnerID: "owner"}).IsOwnedBy("owner") {
		t.Error("IsOwnedBy(owner) = false, want true")
	}
}

func TestNote_Validate(t *testing.T) {
	valid := Note{OwnerID: "owner", Title: "Hello", Body: "Body", Visibility: VisibilityPublic}
	if err := valid.Validate(); err != nil {
		t.Errorf("Validate = %v, want no error", err)
	}
	if err := (Note{OwnerID: "owner", Visibility: "everyone"}).Validate(); err == nil {
		t.Error("Validate with a bad visibility = nil, want an error")
	}
	if err := (Note{OwnerID: "owner", Title: strings.Repeat("a", MaxTitleLength+1), Visibility: VisibilityPublic}).Validate(); err == nil {
		t.Error("Validate with an overlong title = nil, want an error")
	}
	// An ownerless note is unwritable by anyone, so it must never reach a repository.
	if err := (Note{Visibility: VisibilityPublic}).Validate(); err == nil {
		t.Error("Validate with no owner = nil, want an error")
	}
}
