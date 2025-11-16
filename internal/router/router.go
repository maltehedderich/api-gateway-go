package router

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/maltehedderich/api-gateway-go/internal/config"
	"github.com/maltehedderich/api-gateway-go/internal/logger"
)

// Router handles request routing to backend services
type Router struct {
	routes  []*Route
	mu      sync.RWMutex
	logger  *logger.ComponentLogger
}

// Route represents a configured route with compiled pattern
type Route struct {
	PathPattern    string
	CompiledRegex  *regexp.Regexp
	Methods        map[string]bool
	BackendURL     string
	Timeout        int64 // timeout in milliseconds
	AuthPolicy     string
	RequiredRoles  []string
	RateLimits     []config.LimitDefinition
	StripPrefix    string
	Priority       int // Lower number = higher priority
	ParamNames     []string
}

// Match represents a successful route match with extracted parameters
type Match struct {
	Route  *Route
	Params map[string]string
}

// New creates a new router instance
func New() *Router {
	return &Router{
		routes: make([]*Route, 0),
		logger: logger.Get().WithComponent("router"),
	}
}

// LoadRoutes loads routes from configuration
func (r *Router) LoadRoutes(routes []config.RouteConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.routes = make([]*Route, 0, len(routes))

	for i, routeConfig := range routes {
		route, err := r.compileRoute(routeConfig, i)
		if err != nil {
			return fmt.Errorf("failed to compile route %d (%s): %w", i, routeConfig.PathPattern, err)
		}
		r.routes = append(r.routes, route)
	}

	// Sort routes by priority (lower number = higher priority)
	// Routes with exact matches should have higher priority
	r.sortRoutesByPriority()

	r.logger.Info("routes loaded", logger.Fields{
		"count": len(r.routes),
	})

	return nil
}

// compileRoute compiles a route configuration into a Route
func (r *Router) compileRoute(cfg config.RouteConfig, index int) (*Route, error) {
	// Convert path pattern to regex
	pattern, paramNames := r.patternToRegex(cfg.PathPattern)

	compiledRegex, err := regexp.Compile("^" + pattern + "$")
	if err != nil {
		return nil, fmt.Errorf("invalid path pattern: %w", err)
	}

	// Convert methods to map for O(1) lookup
	methods := make(map[string]bool)
	for _, method := range cfg.Methods {
		methods[strings.ToUpper(method)] = true
	}

	// Calculate priority based on pattern specificity
	priority := r.calculatePriority(cfg.PathPattern)

	// Convert timeout to milliseconds
	timeoutMs := int64(cfg.Timeout.Milliseconds())

	route := &Route{
		PathPattern:    cfg.PathPattern,
		CompiledRegex:  compiledRegex,
		Methods:        methods,
		BackendURL:     cfg.BackendURL,
		Timeout:        timeoutMs,
		AuthPolicy:     cfg.AuthPolicy,
		RequiredRoles:  cfg.RequiredRoles,
		RateLimits:     cfg.RateLimits,
		StripPrefix:    cfg.StripPrefix,
		Priority:       priority,
		ParamNames:     paramNames,
	}

	return route, nil
}

// patternToRegex converts a path pattern to a regex pattern
// Supports:
// - Exact match: /api/v1/users
// - Named parameters: /api/v1/users/{id}
// - Wildcards: /api/v1/*
// - Prefix match: /api/v1/**
func (r *Router) patternToRegex(pattern string) (string, []string) {
	paramNames := make([]string, 0)

	// Escape regex special characters except our placeholders
	result := regexp.QuoteMeta(pattern)

	// Replace {param} with named capture groups
	// First, we need to work with the original pattern before QuoteMeta
	// to extract parameter names, then replace in the quoted version
	paramExtractRegex := regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)
	paramMatches := paramExtractRegex.FindAllStringSubmatch(pattern, -1)
	for _, match := range paramMatches {
		if len(match) > 1 {
			paramNames = append(paramNames, match[1])
		}
	}

	// Now replace escaped braces in the result
	paramReplaceRegex := regexp.MustCompile(`\\{[a-zA-Z_][a-zA-Z0-9_]*\\}`)
	result = paramReplaceRegex.ReplaceAllString(result, `([^/]+)`)

	// Replace ** with match everything (greedy)
	result = strings.ReplaceAll(result, `\*\*`, `.*`)

	// Replace * with match segment (non-greedy)
	result = strings.ReplaceAll(result, `\*`, `[^/]*`)

	return result, paramNames
}

// calculatePriority calculates route priority based on pattern specificity
// Lower number = higher priority
// Priority order:
// 1. Exact matches (no parameters or wildcards)
// 2. Paths with parameters
// 3. Paths with single wildcards
// 4. Paths with double wildcards
func (r *Router) calculatePriority(pattern string) int {
	priority := 0

	// Base priority on pattern length (longer = more specific)
	priority = 1000 - len(pattern)

	// Penalize wildcards
	if strings.Contains(pattern, "**") {
		priority += 10000
	} else if strings.Contains(pattern, "*") {
		priority += 5000
	}

	// Penalize parameters
	paramCount := strings.Count(pattern, "{")
	priority += paramCount * 1000

	return priority
}

// sortRoutesByPriority sorts routes by priority
func (r *Router) sortRoutesByPriority() {
	// Simple bubble sort - routes array is typically small
	n := len(r.routes)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if r.routes[j].Priority > r.routes[j+1].Priority {
				r.routes[j], r.routes[j+1] = r.routes[j+1], r.routes[j]
			}
		}
	}
}

// Match finds a matching route for the given request
func (r *Router) Match(req *http.Request) (*Match, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	path := req.URL.Path
	method := req.Method

	// Try to match each route in priority order
	for _, route := range r.routes {
		// Check if method is allowed
		if !route.Methods[method] {
			continue
		}

		// Try to match path pattern
		matches := route.CompiledRegex.FindStringSubmatch(path)
		if matches == nil {
			continue
		}

		// Extract parameters
		params := make(map[string]string)
		for i, paramName := range route.ParamNames {
			if i+1 < len(matches) {
				params[paramName] = matches[i+1]
			}
		}

		r.logger.Debug("route matched", logger.Fields{
			"path":         path,
			"method":       method,
			"pattern":      route.PathPattern,
			"backend_url":  route.BackendURL,
			"params":       params,
		})

		return &Match{
			Route:  route,
			Params: params,
		}, nil
	}

	// No route found
	return nil, fmt.Errorf("no route found for %s %s", method, path)
}

// GetRoutes returns all registered routes (for testing/debugging)
func (r *Router) GetRoutes() []*Route {
	r.mu.RLock()
	defer r.mu.RUnlock()

	routes := make([]*Route, len(r.routes))
	copy(routes, r.routes)
	return routes
}

// Reload reloads routes from configuration
func (r *Router) Reload(routes []config.RouteConfig) error {
	return r.LoadRoutes(routes)
}
