package entity

// This file is the whole of who may do what. Every gate in the service is one question asked of
// it - "may this caller take this action on this thing?" - rather than a rule of its own invented
// where it happens to be needed.
//
// The model has three parts and no more:
//
//   - A Resource is a kind of thing this service holds (a profile, a note).
//   - An Action is one thing that can be done to it (read it, create one, update it, delete it).
//   - An Access is how wide the audience for that pair is: public or private.
//
// The policy table below declares an Access for every (Resource, Action) pair that exists. A pair
// it does not name has no audience at all and is refused, so a feature added without a line here
// is closed rather than open.
//
// There are deliberately no roles: every permission below is decided by who owns the thing, which
// a role would sit between rather than answer.

// Resource is a kind of thing this service holds. It names what a permission is about, and is what
// makes the policy table below readable as a list of everything that can be reached.
type Resource string

const (
	ResourceUser Resource = "user"
	ResourceNote Resource = "note"
)

// Action is one thing that can be done to a resource. These four are the whole vocabulary, and
// they line up with the HTTP methods the routes are registered under - a fifth would mean the API
// grew a verb, not that this list was incomplete.
type Action string

const (
	ActionRead   Action = "read"
	ActionCreate Action = "create"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
)

// Access is how wide the audience for one action on one resource is:
//
//   - Public is everybody, signed in or not.
//   - Private is the owner of the thing and nobody else.
//
// A resource shared with named readers would add a third mode here; nothing needs one yet.
type Access string

const (
	AccessPublic  Access = "public"
	AccessPrivate Access = "private"
)

// Valid reports whether the mode is one this service knows. The zero Access is not: it is what an
// undeclared (resource, action) pair carries, and it denies.
func (a Access) Valid() bool {
	return a == AccessPublic || a == AccessPrivate
}

// policy declares the audience for every action on every resource. It is the one place to read to
// know what this service allows, and the one place to change to alter it.
//
// Some entries are private for a caller who could only ever be acting on their own: a profile is
// addressed as /users/me, and a note is created owned by whoever asked. Those are enforced by the
// address rather than by a check - the forbidden case is unreachable, not refused - and they are
// declared here anyway, because a table that only listed the checks that happen would not say what
// the rules are.
var policy = map[Resource]map[Action]Access{
	// A profile is public because a username is the whole of a public identity: a note shows its
	// author by name. Writing one is the account's own business and nobody else's.
	ResourceUser: {
		ActionRead:   AccessPublic,
		ActionUpdate: AccessPrivate,
		ActionDelete: AccessPrivate,
	},
	// A note's read audience is whatever the note itself says (see Note.Permission). The entry
	// below is the widest a note can be, not what every note is.
	ResourceNote: {
		ActionRead:   AccessPublic,
		ActionCreate: AccessPrivate,
		ActionUpdate: AccessPrivate,
		ActionDelete: AccessPrivate,
	},
}

// Permission is one question about access, ready to be answered: the audience an action on a
// resource has, narrowed to a particular thing by who owns it.
//
// It carries the resource and action it came from so that a refusal can say what was refused, and
// so a permission handed around is self-describing rather than a bare bool with a comment.
type Permission struct {
	Resource Resource
	Action   Action
	Access   Access
	// OwnerID is the uid the thing belongs to - the note's author, the profile's account. It is
	// empty for a resource with no owner, which only a public action can have.
	OwnerID string
}

// PermissionFor returns the declared permission for an action on a resource, before it has been
// narrowed to any particular thing. An undeclared pair comes back with the zero Access, which
// Allows refuses - so reaching for a permission this service does not define denies rather than
// panics or defaults open.
func PermissionFor(resource Resource, action Action) Permission {
	return Permission{Resource: resource, Action: action, Access: policy[resource][action]}
}

// Allows reports whether uid may do the thing this permission describes.
//
// The zero uid is the anonymous caller, and it holds nothing but public access however the rest of
// the permission is filled in: an empty OwnerID must not make a signed-out request the owner of an
// ownerless thing, which is the one way this could have failed open.
func (p Permission) Allows(uid string) bool {
	switch p.Access {
	case AccessPublic:
		return true
	case AccessPrivate:
		return uid != "" && uid == p.OwnerID
	default:
		return false
	}
}
