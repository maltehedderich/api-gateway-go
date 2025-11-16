package auth

import (
	"fmt"
	"sync"
	"time"

	"github.com/maltehedderich/api-gateway-go/internal/logger"
)

// PolicyType represents the type of authorization policy
type PolicyType string

const (
	// PolicyPublic allows unauthenticated access
	PolicyPublic PolicyType = "public"
	// PolicyAuthenticated requires a valid session token
	PolicyAuthenticated PolicyType = "authenticated"
	// PolicyRoleBased requires specific roles
	PolicyRoleBased PolicyType = "role-based"
	// PolicyPermissionBased requires specific permissions
	PolicyPermissionBased PolicyType = "permission-based"
)

// Policy represents an authorization policy
type Policy struct {
	Type        PolicyType
	Roles       []string // Required roles (for role-based policy)
	Permissions []string // Required permissions (for permission-based policy)
	Logic       string   // "AND" or "OR" for multiple requirements
}

// PolicyEvaluator evaluates authorization policies
type PolicyEvaluator struct {
	logger *logger.ComponentLogger
	cache  *policyCache
}

// NewPolicyEvaluator creates a new policy evaluator
func NewPolicyEvaluator(enableCache bool, cacheTTL time.Duration) *PolicyEvaluator {
	var cache *policyCache
	if enableCache {
		cache = newPolicyCache(cacheTTL)
	}

	return &PolicyEvaluator{
		logger: logger.Get().WithComponent("auth.policy"),
		cache:  cache,
	}
}

// Evaluate evaluates a policy against user context
func (pe *PolicyEvaluator) Evaluate(policy *Policy, user *UserContext) (*Decision, error) {
	// Check cache if enabled
	if pe.cache != nil && user != nil {
		cacheKey := pe.buildCacheKey(policy, user)
		if decision, found := pe.cache.get(cacheKey); found {
			pe.logger.Debug("policy decision from cache", logger.Fields{
				"policy_type": policy.Type,
				"user_id":     user.UserID,
				"allowed":     decision.Allowed,
			})
			return decision, nil
		}
	}

	// Evaluate policy
	decision := pe.evaluatePolicy(policy, user)

	// Cache decision if enabled
	if pe.cache != nil && user != nil {
		cacheKey := pe.buildCacheKey(policy, user)
		pe.cache.set(cacheKey, decision)
	}

	pe.logger.Debug("policy evaluated", logger.Fields{
		"policy_type": policy.Type,
		"user_id":     getUserID(user),
		"allowed":     decision.Allowed,
		"reason":      decision.Reason,
	})

	return decision, nil
}

// evaluatePolicy performs the actual policy evaluation
func (pe *PolicyEvaluator) evaluatePolicy(policy *Policy, user *UserContext) *Decision {
	switch policy.Type {
	case PolicyPublic:
		return &Decision{
			Allowed: true,
			Reason:  "public route",
		}

	case PolicyAuthenticated:
		if user == nil {
			return &Decision{
				Allowed: false,
				Reason:  "authentication required",
				Details: map[string]interface{}{
					"required": "valid session token",
				},
			}
		}
		return &Decision{
			Allowed: true,
			Reason:  "authenticated",
		}

	case PolicyRoleBased:
		if user == nil {
			return &Decision{
				Allowed: false,
				Reason:  "authentication required for role-based access",
				Details: map[string]interface{}{
					"required_roles": policy.Roles,
				},
			}
		}
		return pe.evaluateRoleBasedPolicy(policy, user)

	case PolicyPermissionBased:
		if user == nil {
			return &Decision{
				Allowed: false,
				Reason:  "authentication required for permission-based access",
				Details: map[string]interface{}{
					"required_permissions": policy.Permissions,
				},
			}
		}
		return pe.evaluatePermissionBasedPolicy(policy, user)

	default:
		return &Decision{
			Allowed: false,
			Reason:  fmt.Sprintf("unknown policy type: %s", policy.Type),
		}
	}
}

// evaluateRoleBasedPolicy evaluates role-based policy
func (pe *PolicyEvaluator) evaluateRoleBasedPolicy(policy *Policy, user *UserContext) *Decision {
	if len(policy.Roles) == 0 {
		return &Decision{
			Allowed: false,
			Reason:  "no roles specified in policy",
		}
	}

	// Default to OR logic if not specified
	logic := policy.Logic
	if logic == "" {
		logic = "OR"
	}

	if logic == "AND" {
		// User must have ALL required roles
		if user.HasAllRoles(policy.Roles) {
			return &Decision{
				Allowed: true,
				Reason:  "user has all required roles",
			}
		}
		return &Decision{
			Allowed: false,
			Reason:  "insufficient roles",
			Details: map[string]interface{}{
				"required_roles": policy.Roles,
				"user_roles":     user.Roles,
				"logic":          "AND",
			},
		}
	}

	// OR logic - user must have AT LEAST ONE required role
	if user.HasAnyRole(policy.Roles) {
		return &Decision{
			Allowed: true,
			Reason:  "user has required role",
		}
	}

	return &Decision{
		Allowed: false,
		Reason:  "insufficient roles",
		Details: map[string]interface{}{
			"required_roles": policy.Roles,
			"user_roles":     user.Roles,
			"logic":          "OR",
		},
	}
}

// evaluatePermissionBasedPolicy evaluates permission-based policy
func (pe *PolicyEvaluator) evaluatePermissionBasedPolicy(policy *Policy, user *UserContext) *Decision {
	if len(policy.Permissions) == 0 {
		return &Decision{
			Allowed: false,
			Reason:  "no permissions specified in policy",
		}
	}

	// Default to OR logic if not specified
	logic := policy.Logic
	if logic == "" {
		logic = "OR"
	}

	if logic == "AND" {
		// User must have ALL required permissions
		if user.HasAllPermissions(policy.Permissions) {
			return &Decision{
				Allowed: true,
				Reason:  "user has all required permissions",
			}
		}
		return &Decision{
			Allowed: false,
			Reason:  "insufficient permissions",
			Details: map[string]interface{}{
				"required_permissions": policy.Permissions,
				"user_permissions":     user.Permissions,
				"logic":                "AND",
			},
		}
	}

	// OR logic - user must have AT LEAST ONE required permission
	if user.HasAnyPermission(policy.Permissions) {
		return &Decision{
			Allowed: true,
			Reason:  "user has required permission",
		}
	}

	return &Decision{
		Allowed: false,
		Reason:  "insufficient permissions",
		Details: map[string]interface{}{
			"required_permissions": policy.Permissions,
			"user_permissions":     user.Permissions,
			"logic":                "OR",
		},
	}
}

// buildCacheKey builds a cache key for policy decision
func (pe *PolicyEvaluator) buildCacheKey(policy *Policy, user *UserContext) string {
	return fmt.Sprintf("%s:%s:%v", policy.Type, user.UserID, policy)
}

// getUserID safely gets user ID
func getUserID(user *UserContext) string {
	if user == nil {
		return "anonymous"
	}
	return user.UserID
}

// Decision represents an authorization decision
type Decision struct {
	Allowed bool
	Reason  string
	Details map[string]interface{}
}

// policyCache caches authorization decisions
type policyCache struct {
	cache map[string]*cacheEntry
	ttl   time.Duration
	mu    sync.RWMutex
}

type cacheEntry struct {
	decision  *Decision
	expiresAt time.Time
}

// newPolicyCache creates a new policy cache
func newPolicyCache(ttl time.Duration) *policyCache {
	pc := &policyCache{
		cache: make(map[string]*cacheEntry),
		ttl:   ttl,
	}

	// Start cleanup goroutine
	go pc.cleanup()

	return pc
}

// get retrieves a decision from cache
func (pc *policyCache) get(key string) (*Decision, bool) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	entry, found := pc.cache[key]
	if !found {
		return nil, false
	}

	// Check if expired
	if time.Now().After(entry.expiresAt) {
		return nil, false
	}

	return entry.decision, true
}

// set stores a decision in cache
func (pc *policyCache) set(key string, decision *Decision) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.cache[key] = &cacheEntry{
		decision:  decision,
		expiresAt: time.Now().Add(pc.ttl),
	}
}

// cleanup periodically removes expired entries
func (pc *policyCache) cleanup() {
	ticker := time.NewTicker(pc.ttl)
	defer ticker.Stop()

	for range ticker.C {
		pc.mu.Lock()
		now := time.Now()
		for key, entry := range pc.cache {
			if now.After(entry.expiresAt) {
				delete(pc.cache, key)
			}
		}
		pc.mu.Unlock()
	}
}
