package authclient

import "context"

// Role is the user authorization role.
//
// Values mirror auth.v1 proto contract. New roles are additive after v1.0.0;
// adding a new constant must NOT renumber the existing ones.
type Role string

const (
	// RoleUnknown is populated when AUTH returns an unrecognized role.
	RoleUnknown Role = ""

	// RoleUser is the standard authenticated user role.
	RoleUser Role = "user"

	// RoleAdmin is the privileged administrator role.
	RoleAdmin Role = "admin"
)

// User is the authenticated principal extracted from a valid access token.
//
// Email is populated only by Login and GetCurrentUser. After
// ValidateAccessToken the Email is always empty because the gRPC
// ValidateTokenResponse contract does not carry it — fetch the full
// profile via GetCurrentUser when an email is required.
type User struct {
	UUID  string
	Role  Role
	Email string
}

// IsZero reports whether u is the zero value (no UUID).
func (u User) IsZero() bool { return u.UUID == "" }

// IsAdmin reports whether u has the admin role.
func (u User) IsAdmin() bool { return u.Role == RoleAdmin }

// HasRole reports whether u has the exact given role.
func (u User) HasRole(role Role) bool { return u.Role == role }

// userCtxKey is the unexported context key for User values stored by WithUser.
// Using a distinct unexported type avoids collisions with other packages.
type userCtxKey struct{}

// WithUser returns a copy of ctx with user attached. Retrieve it with
// UserFromContext.
func WithUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, userCtxKey{}, user)
}

// UserFromContext extracts the User stored by WithUser. The boolean is false
// when no User is attached to ctx.
func UserFromContext(ctx context.Context) (User, bool) {
	user, ok := ctx.Value(userCtxKey{}).(User)
	return user, ok
}
