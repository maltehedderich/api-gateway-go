package circuitbreaker

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/maltehedderich/api-gateway-go/internal/logger"
	"github.com/maltehedderich/api-gateway-go/internal/metrics"
)

// State represents the circuit breaker state
type State int

const (
	// StateClosed means requests are allowed
	StateClosed State = iota
	// StateOpen means requests are blocked
	StateOpen
	// StateHalfOpen means limited requests are allowed to test recovery
	StateHalfOpen
)

// String returns the string representation of the state
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

var (
	// ErrCircuitOpen is returned when the circuit breaker is open
	ErrCircuitOpen = errors.New("circuit breaker is open")
)

// Config contains circuit breaker configuration
type Config struct {
	// FailureThreshold is the number of consecutive failures before opening
	FailureThreshold int
	// SuccessThreshold is the number of consecutive successes in half-open before closing
	SuccessThreshold int
	// Timeout is how long to wait in open state before trying half-open
	Timeout time.Duration
	// MaxRequests is the maximum number of requests allowed in half-open state
	MaxRequests int
}

// DefaultConfig returns default circuit breaker configuration
func DefaultConfig() *Config {
	return &Config{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          60 * time.Second,
		MaxRequests:      3,
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	name            string
	config          *Config
	state           State
	failures        int
	successes       int
	lastFailureTime time.Time
	lastStateChange time.Time
	halfOpenRequests int
	mu              sync.RWMutex
	logger          *logger.ComponentLogger
}

// New creates a new circuit breaker
func New(name string, config *Config) *CircuitBreaker {
	if config == nil {
		config = DefaultConfig()
	}

	return &CircuitBreaker{
		name:            name,
		config:          config,
		state:           StateClosed,
		lastStateChange: time.Now(),
		logger:          logger.Get().WithComponent("circuitbreaker"),
	}
}

// Execute executes a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(fn func() error) error {
	// Check if request is allowed
	if err := cb.beforeRequest(); err != nil {
		return err
	}

	// Execute function
	err := fn()

	// Record result
	cb.afterRequest(err)

	return err
}

// beforeRequest checks if the request is allowed
func (cb *CircuitBreaker) beforeRequest() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		// Allow request
		return nil

	case StateOpen:
		// Check if timeout has elapsed
		if time.Since(cb.lastStateChange) >= cb.config.Timeout {
			// Transition to half-open
			cb.setState(StateHalfOpen)
			cb.halfOpenRequests = 0
			return nil
		}
		// Circuit is still open
		return ErrCircuitOpen

	case StateHalfOpen:
		// Allow limited requests
		if cb.halfOpenRequests >= cb.config.MaxRequests {
			return ErrCircuitOpen
		}
		cb.halfOpenRequests++
		return nil

	default:
		return ErrCircuitOpen
	}
}

// afterRequest records the result of a request
func (cb *CircuitBreaker) afterRequest(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.onFailure()
	} else {
		cb.onSuccess()
	}
}

// onFailure handles a failed request
func (cb *CircuitBreaker) onFailure() {
	cb.failures++
	cb.successes = 0
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		if cb.failures >= cb.config.FailureThreshold {
			cb.setState(StateOpen)
		}

	case StateHalfOpen:
		// Any failure in half-open goes back to open
		cb.setState(StateOpen)
	}
}

// onSuccess handles a successful request
func (cb *CircuitBreaker) onSuccess() {
	cb.successes++

	switch cb.state {
	case StateClosed:
		// Reset failure count on success
		cb.failures = 0

	case StateHalfOpen:
		if cb.successes >= cb.config.SuccessThreshold {
			// Transition to closed
			cb.setState(StateClosed)
			cb.failures = 0
			cb.halfOpenRequests = 0
		}
	}
}

// setState changes the circuit breaker state
func (cb *CircuitBreaker) setState(newState State) {
	if cb.state == newState {
		return
	}

	oldState := cb.state
	cb.state = newState
	cb.lastStateChange = time.Now()

	// Record metrics
	metrics.SetCircuitBreakerState(cb.name, int(newState))
	metrics.RecordCircuitBreakerTransition(cb.name, oldState.String(), newState.String())

	cb.logger.Info("circuit breaker state changed", logger.Fields{
		"name":      cb.name,
		"old_state": oldState.String(),
		"new_state": newState.String(),
		"failures":  cb.failures,
		"successes": cb.successes,
	})
}

// GetState returns the current state
func (cb *CircuitBreaker) GetState() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats returns current statistics
func (cb *CircuitBreaker) GetStats() Stats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return Stats{
		Name:            cb.name,
		State:           cb.state,
		Failures:        cb.failures,
		Successes:       cb.successes,
		LastFailureTime: cb.lastFailureTime,
		LastStateChange: cb.lastStateChange,
	}
}

// Stats contains circuit breaker statistics
type Stats struct {
	Name            string
	State           State
	Failures        int
	Successes       int
	LastFailureTime time.Time
	LastStateChange time.Time
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.failures = 0
	cb.successes = 0
	cb.halfOpenRequests = 0
	cb.lastStateChange = time.Now()

	cb.logger.Info("circuit breaker reset", logger.Fields{
		"name": cb.name,
	})
}

// Manager manages multiple circuit breakers
type Manager struct {
	breakers map[string]*CircuitBreaker
	mu       sync.RWMutex
	logger   *logger.ComponentLogger
}

// NewManager creates a new circuit breaker manager
func NewManager() *Manager {
	return &Manager{
		breakers: make(map[string]*CircuitBreaker),
		logger:   logger.Get().WithComponent("circuitbreaker.manager"),
	}
}

// Get gets or creates a circuit breaker for a backend
func (m *Manager) Get(name string, config *Config) *CircuitBreaker {
	m.mu.RLock()
	cb, exists := m.breakers[name]
	m.mu.RUnlock()

	if exists {
		return cb
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if cb, exists := m.breakers[name]; exists {
		return cb
	}

	// Create new circuit breaker
	cb = New(name, config)
	m.breakers[name] = cb

	m.logger.Info("circuit breaker created", logger.Fields{
		"name": name,
	})

	return cb
}

// GetStats returns statistics for all circuit breakers
func (m *Manager) GetStats() []Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make([]Stats, 0, len(m.breakers))
	for _, cb := range m.breakers {
		stats = append(stats, cb.GetStats())
	}

	return stats
}

// Reset resets a specific circuit breaker
func (m *Manager) Reset(name string) error {
	m.mu.RLock()
	cb, exists := m.breakers[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("circuit breaker not found: %s", name)
	}

	cb.Reset()
	return nil
}

// ResetAll resets all circuit breakers
func (m *Manager) ResetAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, cb := range m.breakers {
		cb.Reset()
	}

	m.logger.Info("all circuit breakers reset")
}
