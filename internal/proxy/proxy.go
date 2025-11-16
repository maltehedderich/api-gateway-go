package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/maltehedderich/api-gateway-go/internal/circuitbreaker"
	"github.com/maltehedderich/api-gateway-go/internal/logger"
	"github.com/maltehedderich/api-gateway-go/internal/router"
)

// Proxy handles request forwarding to backend services
type Proxy struct {
	client          *http.Client
	logger          *logger.ComponentLogger
	config          *Config
	circuitBreakers *circuitbreaker.Manager
}

// Config contains proxy configuration
type Config struct {
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	IdleConnTimeout     time.Duration
	DefaultTimeout      time.Duration
	MaxRetries          int
	RetryDelay          time.Duration
}

// DefaultConfig returns default proxy configuration
func DefaultConfig() *Config {
	return &Config{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DefaultTimeout:      30 * time.Second,
		MaxRetries:          3,
		RetryDelay:          100 * time.Millisecond,
	}
}

// New creates a new proxy instance
func New(config *Config) *Proxy {
	if config == nil {
		config = DefaultConfig()
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        config.MaxIdleConns,
		MaxIdleConnsPerHost: config.MaxIdleConnsPerHost,
		IdleConnTimeout:     config.IdleConnTimeout,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   config.DefaultTimeout,
		// Don't follow redirects - let the client handle them
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return &Proxy{
		client:          client,
		logger:          logger.Get().WithComponent("proxy"),
		config:          config,
		circuitBreakers: circuitbreaker.NewManager(),
	}
}

// Forward forwards a request to the backend service
func (p *Proxy) Forward(w http.ResponseWriter, r *http.Request, match *router.Match) error {
	// Parse backend URL
	backendURL, err := url.Parse(match.Route.BackendURL)
	if err != nil {
		return fmt.Errorf("invalid backend URL: %w", err)
	}

	// Build target URL
	targetURL := p.buildTargetURL(backendURL, r, match)

	// Create backend request
	backendReq, err := p.createBackendRequest(r, targetURL, match)
	if err != nil {
		return fmt.Errorf("failed to create backend request: %w", err)
	}

	// Set timeout if specified in route
	if match.Route.Timeout > 0 {
		timeout := time.Duration(match.Route.Timeout) * time.Millisecond
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		backendReq = backendReq.WithContext(ctx)
	}

	// Get circuit breaker for this backend
	cb := p.circuitBreakers.Get(match.Route.BackendURL, circuitbreaker.DefaultConfig())

	// Execute request with circuit breaker protection
	var resp *http.Response
	err = cb.Execute(func() error {
		var execErr error
		resp, execErr = p.forwardWithRetry(backendReq)
		return execErr
	})

	if err != nil {
		if err == circuitbreaker.ErrCircuitOpen {
			return fmt.Errorf("circuit breaker open for backend %s", match.Route.BackendURL)
		}
		return fmt.Errorf("backend request failed: %w", err)
	}
	defer resp.Body.Close()

	// Log backend response
	correlationID := logger.GetCorrelationID(r.Context())
	p.logger.Debug("backend response received", logger.Fields{
		"correlation_id": correlationID,
		"backend_url":    targetURL.String(),
		"status":         resp.StatusCode,
		"content_length": resp.ContentLength,
	})

	// Copy response headers
	p.copyResponseHeaders(w, resp)

	// Copy status code
	w.WriteHeader(resp.StatusCode)

	// Stream response body
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		p.logger.Warn("error streaming response", logger.Fields{
			"correlation_id": correlationID,
			"error":          err.Error(),
		})
	}

	return nil
}

// buildTargetURL builds the target backend URL
func (p *Proxy) buildTargetURL(backendURL *url.URL, r *http.Request, match *router.Match) *url.URL {
	targetURL := &url.URL{
		Scheme: backendURL.Scheme,
		Host:   backendURL.Host,
	}

	// Handle path construction
	path := r.URL.Path

	// Strip prefix if configured
	if match.Route.StripPrefix != "" && strings.HasPrefix(path, match.Route.StripPrefix) {
		path = strings.TrimPrefix(path, match.Route.StripPrefix)
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
	}

	// Combine backend path with request path
	if backendURL.Path != "" && backendURL.Path != "/" {
		targetURL.Path = backendURL.Path + path
	} else {
		targetURL.Path = path
	}

	// Copy query parameters
	targetURL.RawQuery = r.URL.RawQuery

	return targetURL
}

// createBackendRequest creates a new HTTP request for the backend
func (p *Proxy) createBackendRequest(r *http.Request, targetURL *url.URL, match *router.Match) (*http.Request, error) {
	// Create new request with same method and body
	backendReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL.String(), r.Body)
	if err != nil {
		return nil, err
	}

	// Copy headers, excluding hop-by-hop headers
	p.copyRequestHeaders(backendReq, r)

	// Add X-Forwarded-* headers
	p.addForwardedHeaders(backendReq, r)

	// Add correlation ID header
	correlationID := logger.GetCorrelationID(r.Context())
	if correlationID != "" {
		backendReq.Header.Set("X-Correlation-ID", correlationID)
	}

	// Add Via header
	backendReq.Header.Add("Via", "1.1 gateway")

	// Set Host header to backend host
	backendReq.Host = targetURL.Host

	return backendReq, nil
}

// copyRequestHeaders copies request headers, excluding hop-by-hop headers
func (p *Proxy) copyRequestHeaders(dst, src *http.Request) {
	// Hop-by-hop headers that should not be forwarded
	hopHeaders := map[string]bool{
		"Connection":          true,
		"Keep-Alive":          true,
		"Proxy-Authenticate":  true,
		"Proxy-Authorization": true,
		"Te":                  true,
		"Trailer":             true,
		"Transfer-Encoding":   true,
		"Upgrade":             true,
	}

	for key, values := range src.Header {
		// Skip hop-by-hop headers
		if hopHeaders[key] {
			continue
		}

		// Copy header values
		for _, value := range values {
			dst.Header.Add(key, value)
		}
	}
}

// addForwardedHeaders adds X-Forwarded-* headers
func (p *Proxy) addForwardedHeaders(backendReq, originalReq *http.Request) {
	// X-Forwarded-For
	clientIP := p.getClientIP(originalReq)
	if prior := originalReq.Header.Get("X-Forwarded-For"); prior != "" {
		clientIP = prior + ", " + clientIP
	}
	backendReq.Header.Set("X-Forwarded-For", clientIP)

	// X-Forwarded-Proto
	proto := "http"
	if originalReq.TLS != nil {
		proto = "https"
	}
	backendReq.Header.Set("X-Forwarded-Proto", proto)

	// X-Forwarded-Host
	backendReq.Header.Set("X-Forwarded-Host", originalReq.Host)

	// X-Real-IP (if not already set)
	if originalReq.Header.Get("X-Real-IP") == "" {
		backendReq.Header.Set("X-Real-IP", p.getClientIP(originalReq))
	}
}

// getClientIP extracts the client IP from the request
func (p *Proxy) getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// X-Forwarded-For can contain multiple IPs, get the first one
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return ip
}

// copyResponseHeaders copies response headers
func (p *Proxy) copyResponseHeaders(dst http.ResponseWriter, src *http.Response) {
	// Hop-by-hop headers that should not be forwarded
	hopHeaders := map[string]bool{
		"Connection":        true,
		"Keep-Alive":        true,
		"Proxy-Authenticate": true,
		"Proxy-Authorization": true,
		"Te":                true,
		"Trailer":           true,
		"Transfer-Encoding": true,
		"Upgrade":           true,
	}

	for key, values := range src.Header {
		// Skip hop-by-hop headers
		if hopHeaders[key] {
			continue
		}

		// Copy header values
		for _, value := range values {
			dst.Header().Add(key, value)
		}
	}

	// Add X-Gateway-Version header
	dst.Header().Set("X-Gateway-Version", "1.0.0")
}

// forwardWithRetry forwards the request with retry logic
func (p *Proxy) forwardWithRetry(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retrying with exponential backoff
			delay := p.config.RetryDelay * time.Duration(1<<uint(attempt-1))
			time.Sleep(delay)

			p.logger.Debug("retrying backend request", logger.Fields{
				"attempt": attempt,
				"url":     req.URL.String(),
				"delay":   delay.String(),
			})
		}

		// Execute request
		resp, err = p.client.Do(req)

		// If successful or non-retryable error, return
		if err == nil {
			// Don't retry on successful responses (even 4xx/5xx)
			// The client should handle those
			return resp, nil
		}

		// Check if error is retryable
		if !p.isRetryable(err) {
			return nil, err
		}

		// Log retry
		correlationID := logger.GetCorrelationID(req.Context())
		p.logger.Warn("backend request failed, will retry", logger.Fields{
			"correlation_id": correlationID,
			"attempt":        attempt,
			"error":          err.Error(),
		})
	}

	return nil, fmt.Errorf("max retries exceeded: %w", err)
}

// isRetryable checks if an error is retryable
func (p *Proxy) isRetryable(err error) bool {
	// Network errors are retryable
	if _, ok := err.(net.Error); ok {
		return true
	}

	// Timeout errors are retryable
	if err == context.DeadlineExceeded {
		return true
	}

	// Connection refused is retryable
	if strings.Contains(err.Error(), "connection refused") {
		return true
	}

	// DNS errors are retryable
	if strings.Contains(err.Error(), "no such host") {
		return true
	}

	return false
}
