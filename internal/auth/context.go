package auth

import (
	"context"
)

// ContextKey is a type for context keys to avoid collisions
type ContextKey string

const (
	// UserContextKey is the context key for user information
	UserContextKey ContextKey = "auth_user"
)

// UserContext represents authenticated user information stored in request context
type UserContext struct {
	UserID      string
	SessionID   string
	Roles       []string
	Permissions []string
	Claims      *Claims
}

// SetUserContext stores user context in the request context
func SetUserContext(ctx context.Context, user *UserContext) context.Context {
	return context.WithValue(ctx, UserContextKey, user)
}

// GetUserContext retrieves user context from the request context
func GetUserContext(ctx context.Context) (*UserContext, bool) {
	user, ok := ctx.Value(UserContextKey).(*UserContext)
	return user, ok
}

// NewUserContext creates a new user context from validated claims
func NewUserContext(claims *Claims) *UserContext {
	return &UserContext{
		UserID:      claims.UserID,
		SessionID:   claims.SessionID,
		Roles:       claims.Roles,
		Permissions: claims.Permissions,
		Claims:      claims,
	}
}

// HasRole checks if the user has a specific role
func (uc *UserContext) HasRole(role string) bool {
	for _, r := range uc.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasAnyRole checks if the user has any of the specified roles
func (uc *UserContext) HasAnyRole(roles []string) bool {
	for _, role := range roles {
		if uc.HasRole(role) {
			return true
		}
	}
	return false
}

// HasAllRoles checks if the user has all of the specified roles
func (uc *UserContext) HasAllRoles(roles []string) bool {
	for _, role := range roles {
		if !uc.HasRole(role) {
			return false
		}
	}
	return true
}

// HasPermission checks if the user has a specific permission
func (uc *UserContext) HasPermission(permission string) bool {
	for _, p := range uc.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

// HasAnyPermission checks if the user has any of the specified permissions
func (uc *UserContext) HasAnyPermission(permissions []string) bool {
	for _, permission := range permissions {
		if uc.HasPermission(permission) {
			return true
		}
	}
	return false
}

// HasAllPermissions checks if the user has all of the specified permissions
func (uc *UserContext) HasAllPermissions(permissions []string) bool {
	for _, permission := range permissions {
		if !uc.HasPermission(permission) {
			return false
		}
	}
	return true
}
