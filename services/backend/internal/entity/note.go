package entity

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxTitleLength = 200
	MaxBodyLength  = 100_000
)

// Visibility decides who may read a note beyond its owner.
type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
)

func (v Visibility) Valid() bool {
	return v == VisibilityPublic || v == VisibilityPrivate
}

// Note is the template's example resource: something a signed-in caller owns and chooses the
// audience of. This service holds the only credentials for the collection, so read and write
// access is decided here (Permission, and CanBeReadBy over it) and nowhere else.
//
// ID is assigned by the repository at creation and never changes.
type Note struct {
	ID         string
	OwnerID    string
	Title      string
	Body       string
	Visibility Visibility
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// SetTitle trims and validates a new title before applying it. An empty title is allowed - the
// frontend renders those as "(untitled)".
func (n *Note) SetTitle(title string) error {
	trimmed := strings.TrimSpace(title)
	if utf8.RuneCountInString(trimmed) > MaxTitleLength {
		return ValidationError{Field: "title", Message: lengthMessage(MaxTitleLength)}
	}

	n.Title = trimmed
	return nil
}

// SetBody validates a new body before applying it. Whitespace is significant here, so unlike the
// other setters this one does not trim.
func (n *Note) SetBody(body string) error {
	if utf8.RuneCountInString(body) > MaxBodyLength {
		return ValidationError{Field: "body", Message: lengthMessage(MaxBodyLength)}
	}

	n.Body = body
	return nil
}

// SetVisibility validates a new visibility before applying it.
func (n *Note) SetVisibility(visibility Visibility) error {
	if !visibility.Valid() {
		return ValidationError{
			Field:   "visibility",
			Message: fmt.Sprintf("must be %q or %q", VisibilityPublic, VisibilityPrivate),
		}
	}

	n.Visibility = visibility
	return nil
}

// Validate reports whether the note is in a storable state: the server-set owner is present, and
// every other field holds a value the setters above would accept. Repositories call it before
// each write, so a note assembled outside the HTTP layer cannot sidestep the rules.
func (n Note) Validate() error {
	if n.OwnerID == "" {
		return ValidationError{Field: "ownerId", Message: "is required"}
	}

	candidate := n
	if err := candidate.SetTitle(n.Title); err != nil {
		return err
	}
	if err := candidate.SetBody(n.Body); err != nil {
		return err
	}
	return candidate.SetVisibility(n.Visibility)
}

// IsOwnedBy reports whether uid wrote this note.
func (n Note) IsOwnedBy(uid string) bool {
	return n.OwnerID != "" && n.OwnerID == uid
}

// Permission is the single definition of who may do what to a note: reading follows the note's
// own visibility, and writing it - creating, updating, deleting - is the owner's alone.
//
// Anything but "public" reads as private, so a document holding a visibility this build does not
// know is closed rather than open.
func (n Note) Permission(action Action) Permission {
	permission := PermissionFor(ResourceNote, action)
	permission.OwnerID = n.OwnerID
	if action == ActionRead && n.Visibility != VisibilityPublic {
		permission.Access = AccessPrivate
	}
	return permission
}

// CanBeReadBy reports whether uid may see this note. Repositories use it to filter what they hand
// back, so it stays a method on the note rather than something every caller assembles.
func (n Note) CanBeReadBy(uid string) bool {
	return n.Permission(ActionRead).Allows(uid)
}
