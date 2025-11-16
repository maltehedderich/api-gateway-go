package auth

import (
	"context"
	"testing"
)

func TestUserContext_HasRole(t *testing.T) {
	user := &UserContext{
		UserID:    "user123",
		SessionID: "session456",
		Roles:     []string{"admin", "user", "moderator"},
	}

	tests := []struct {
		name     string
		role     string
		expected bool
	}{
		{"HasRole_Admin", "admin", true},
		{"HasRole_User", "user", true},
		{"HasRole_Moderator", "moderator", true},
		{"HasRole_NotPresent", "superadmin", false},
		{"HasRole_Empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := user.HasRole(tt.role)
			if result != tt.expected {
				t.Errorf("HasRole(%s) = %v, want %v", tt.role, result, tt.expected)
			}
		})
	}
}

func TestUserContext_HasAnyRole(t *testing.T) {
	user := &UserContext{
		UserID:    "user123",
		SessionID: "session456",
		Roles:     []string{"admin", "user"},
	}

	tests := []struct {
		name     string
		roles    []string
		expected bool
	}{
		{"HasAnyRole_OneMatch", []string{"admin", "superadmin"}, true},
		{"HasAnyRole_MultipleMatches", []string{"admin", "user"}, true},
		{"HasAnyRole_NoMatch", []string{"superadmin", "moderator"}, false},
		{"HasAnyRole_Empty", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := user.HasAnyRole(tt.roles)
			if result != tt.expected {
				t.Errorf("HasAnyRole(%v) = %v, want %v", tt.roles, result, tt.expected)
			}
		})
	}
}

func TestUserContext_HasAllRoles(t *testing.T) {
	user := &UserContext{
		UserID:    "user123",
		SessionID: "session456",
		Roles:     []string{"admin", "user", "moderator"},
	}

	tests := []struct {
		name     string
		roles    []string
		expected bool
	}{
		{"HasAllRoles_AllMatch", []string{"admin", "user"}, true},
		{"HasAllRoles_Single", []string{"admin"}, true},
		{"HasAllRoles_PartialMatch", []string{"admin", "superadmin"}, false},
		{"HasAllRoles_NoMatch", []string{"superadmin", "editor"}, false},
		{"HasAllRoles_Empty", []string{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := user.HasAllRoles(tt.roles)
			if result != tt.expected {
				t.Errorf("HasAllRoles(%v) = %v, want %v", tt.roles, result, tt.expected)
			}
		})
	}
}

func TestUserContext_HasPermission(t *testing.T) {
	user := &UserContext{
		UserID:      "user123",
		SessionID:   "session456",
		Permissions: []string{"read:orders", "write:orders", "read:users"},
	}

	tests := []struct {
		name       string
		permission string
		expected   bool
	}{
		{"HasPermission_ReadOrders", "read:orders", true},
		{"HasPermission_WriteOrders", "write:orders", true},
		{"HasPermission_NotPresent", "delete:orders", false},
		{"HasPermission_Empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := user.HasPermission(tt.permission)
			if result != tt.expected {
				t.Errorf("HasPermission(%s) = %v, want %v", tt.permission, result, tt.expected)
			}
		})
	}
}

func TestUserContext_HasAnyPermission(t *testing.T) {
	user := &UserContext{
		UserID:      "user123",
		SessionID:   "session456",
		Permissions: []string{"read:orders", "write:orders"},
	}

	tests := []struct {
		name        string
		permissions []string
		expected    bool
	}{
		{"HasAnyPermission_OneMatch", []string{"read:orders", "delete:orders"}, true},
		{"HasAnyPermission_MultipleMatches", []string{"read:orders", "write:orders"}, true},
		{"HasAnyPermission_NoMatch", []string{"delete:orders", "admin:all"}, false},
		{"HasAnyPermission_Empty", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := user.HasAnyPermission(tt.permissions)
			if result != tt.expected {
				t.Errorf("HasAnyPermission(%v) = %v, want %v", tt.permissions, result, tt.expected)
			}
		})
	}
}

func TestUserContext_HasAllPermissions(t *testing.T) {
	user := &UserContext{
		UserID:      "user123",
		SessionID:   "session456",
		Permissions: []string{"read:orders", "write:orders", "read:users"},
	}

	tests := []struct {
		name        string
		permissions []string
		expected    bool
	}{
		{"HasAllPermissions_AllMatch", []string{"read:orders", "write:orders"}, true},
		{"HasAllPermissions_Single", []string{"read:orders"}, true},
		{"HasAllPermissions_PartialMatch", []string{"read:orders", "delete:orders"}, false},
		{"HasAllPermissions_NoMatch", []string{"delete:orders", "admin:all"}, false},
		{"HasAllPermissions_Empty", []string{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := user.HasAllPermissions(tt.permissions)
			if result != tt.expected {
				t.Errorf("HasAllPermissions(%v) = %v, want %v", tt.permissions, result, tt.expected)
			}
		})
	}
}

func TestSetAndGetUserContext(t *testing.T) {
	ctx := context.Background()

	user := &UserContext{
		UserID:    "user123",
		SessionID: "session456",
		Roles:     []string{"admin"},
	}

	// Set user context
	ctx = SetUserContext(ctx, user)

	// Get user context
	retrievedUser, ok := GetUserContext(ctx)
	if !ok {
		t.Error("Expected to retrieve user context, but got false")
	}

	if retrievedUser.UserID != user.UserID {
		t.Errorf("Expected UserID %s, got %s", user.UserID, retrievedUser.UserID)
	}

	if retrievedUser.SessionID != user.SessionID {
		t.Errorf("Expected SessionID %s, got %s", user.SessionID, retrievedUser.SessionID)
	}
}

func TestGetUserContext_NotPresent(t *testing.T) {
	ctx := context.Background()

	// Try to get user context when it's not set
	_, ok := GetUserContext(ctx)
	if ok {
		t.Error("Expected GetUserContext to return false when user context is not set")
	}
}

func TestNewUserContext(t *testing.T) {
	claims := &Claims{
		UserID:      "user123",
		SessionID:   "session456",
		Roles:       []string{"admin", "user"},
		Permissions: []string{"read:orders", "write:orders"},
	}

	userCtx := NewUserContext(claims)

	if userCtx.UserID != claims.UserID {
		t.Errorf("Expected UserID %s, got %s", claims.UserID, userCtx.UserID)
	}

	if userCtx.SessionID != claims.SessionID {
		t.Errorf("Expected SessionID %s, got %s", claims.SessionID, userCtx.SessionID)
	}

	if len(userCtx.Roles) != len(claims.Roles) {
		t.Errorf("Expected %d roles, got %d", len(claims.Roles), len(userCtx.Roles))
	}

	if len(userCtx.Permissions) != len(claims.Permissions) {
		t.Errorf("Expected %d permissions, got %d", len(claims.Permissions), len(userCtx.Permissions))
	}

	if userCtx.Claims != claims {
		t.Error("Expected Claims to be set correctly")
	}
}
