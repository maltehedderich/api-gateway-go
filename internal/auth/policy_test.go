package auth

import (
	"testing"
	"time"
)

func TestPolicyEvaluator_Evaluate(t *testing.T) {
	evaluator := NewPolicyEvaluator(false, 5*time.Minute)

	t.Run("PublicPolicy", func(t *testing.T) {
		policy := &Policy{
			Type: PolicyPublic,
		}

		// Should allow without user context
		decision, err := evaluator.Evaluate(policy, nil)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if !decision.Allowed {
			t.Error("Expected public policy to allow access")
		}
	})

	t.Run("AuthenticatedPolicy_WithUser", func(t *testing.T) {
		policy := &Policy{
			Type: PolicyAuthenticated,
		}

		user := &UserContext{
			UserID:    "user123",
			SessionID: "session456",
		}

		decision, err := evaluator.Evaluate(policy, user)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if !decision.Allowed {
			t.Error("Expected authenticated policy to allow access with valid user")
		}
	})

	t.Run("AuthenticatedPolicy_WithoutUser", func(t *testing.T) {
		policy := &Policy{
			Type: PolicyAuthenticated,
		}

		decision, err := evaluator.Evaluate(policy, nil)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if decision.Allowed {
			t.Error("Expected authenticated policy to deny access without user")
		}

		if decision.Reason != "authentication required" {
			t.Errorf("Expected reason 'authentication required', got: %s", decision.Reason)
		}
	})

	t.Run("RoleBasedPolicy_OR_Logic", func(t *testing.T) {
		policy := &Policy{
			Type:  PolicyRoleBased,
			Roles: []string{"admin", "moderator"},
			Logic: "OR",
		}

		// User with admin role
		user := &UserContext{
			UserID: "user123",
			Roles:  []string{"admin", "user"},
		}

		decision, err := evaluator.Evaluate(policy, user)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if !decision.Allowed {
			t.Error("Expected role-based policy to allow access when user has one required role")
		}
	})

	t.Run("RoleBasedPolicy_OR_Logic_NoMatch", func(t *testing.T) {
		policy := &Policy{
			Type:  PolicyRoleBased,
			Roles: []string{"admin", "moderator"},
			Logic: "OR",
		}

		// User without required roles
		user := &UserContext{
			UserID: "user123",
			Roles:  []string{"user", "viewer"},
		}

		decision, err := evaluator.Evaluate(policy, user)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if decision.Allowed {
			t.Error("Expected role-based policy to deny access when user has no required roles")
		}
	})

	t.Run("RoleBasedPolicy_AND_Logic", func(t *testing.T) {
		policy := &Policy{
			Type:  PolicyRoleBased,
			Roles: []string{"admin", "moderator"},
			Logic: "AND",
		}

		// User with both required roles
		user := &UserContext{
			UserID: "user123",
			Roles:  []string{"admin", "moderator", "user"},
		}

		decision, err := evaluator.Evaluate(policy, user)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if !decision.Allowed {
			t.Error("Expected role-based policy to allow access when user has all required roles")
		}
	})

	t.Run("RoleBasedPolicy_AND_Logic_Partial", func(t *testing.T) {
		policy := &Policy{
			Type:  PolicyRoleBased,
			Roles: []string{"admin", "moderator"},
			Logic: "AND",
		}

		// User with only one required role
		user := &UserContext{
			UserID: "user123",
			Roles:  []string{"admin", "user"},
		}

		decision, err := evaluator.Evaluate(policy, user)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if decision.Allowed {
			t.Error("Expected role-based policy to deny access when user doesn't have all required roles")
		}
	})

	t.Run("PermissionBasedPolicy_OR_Logic", func(t *testing.T) {
		policy := &Policy{
			Type:        PolicyPermissionBased,
			Permissions: []string{"read:orders", "write:orders"},
			Logic:       "OR",
		}

		// User with one required permission
		user := &UserContext{
			UserID:      "user123",
			Permissions: []string{"read:orders", "read:users"},
		}

		decision, err := evaluator.Evaluate(policy, user)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if !decision.Allowed {
			t.Error("Expected permission-based policy to allow access when user has one required permission")
		}
	})

	t.Run("PermissionBasedPolicy_AND_Logic", func(t *testing.T) {
		policy := &Policy{
			Type:        PolicyPermissionBased,
			Permissions: []string{"read:orders", "write:orders"},
			Logic:       "AND",
		}

		// User with both required permissions
		user := &UserContext{
			UserID:      "user123",
			Permissions: []string{"read:orders", "write:orders", "read:users"},
		}

		decision, err := evaluator.Evaluate(policy, user)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if !decision.Allowed {
			t.Error("Expected permission-based policy to allow access when user has all required permissions")
		}
	})
}

func TestPolicyEvaluator_Cache(t *testing.T) {
	// Create evaluator with caching enabled
	evaluator := NewPolicyEvaluator(true, 100*time.Millisecond)

	policy := &Policy{
		Type: PolicyAuthenticated,
	}

	user := &UserContext{
		UserID:    "user123",
		SessionID: "session456",
	}

	// First evaluation
	decision1, err := evaluator.Evaluate(policy, user)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Second evaluation (should be from cache)
	decision2, err := evaluator.Evaluate(policy, user)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if decision1.Allowed != decision2.Allowed {
		t.Error("Expected cached decision to match original decision")
	}

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Third evaluation (cache should be expired)
	decision3, err := evaluator.Evaluate(policy, user)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if decision3.Allowed != decision1.Allowed {
		t.Error("Expected decision after cache expiry to match original decision")
	}
}
