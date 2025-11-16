package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/maltehedderich/api-gateway-go/internal/auth"
	"github.com/maltehedderich/api-gateway-go/internal/config"
	"github.com/maltehedderich/api-gateway-go/internal/health"
	"github.com/maltehedderich/api-gateway-go/internal/logger"
	"github.com/maltehedderich/api-gateway-go/internal/middleware"
	"github.com/maltehedderich/api-gateway-go/internal/proxy"
	"github.com/maltehedderich/api-gateway-go/internal/ratelimit"
	"github.com/maltehedderich/api-gateway-go/internal/router"
)

// Server represents the API Gateway server
type Server struct {
	config        *config.Config
	httpServer    *http.Server
	httpsServer   *http.Server
	healthManager *health.Manager
	router        *router.Router
	proxy         *proxy.Proxy
	rateLimiter   *ratelimit.Limiter
	authMiddleware *auth.Middleware
	logger        *logger.ComponentLogger
}

// New creates a new server instance
func New(cfg *config.Config, healthMgr *health.Manager) *Server {
	log := logger.Get().WithComponent("server")

	// Create router
	rtr := router.New()

	// Load routes from configuration
	if err := rtr.LoadRoutes(cfg.Routes); err != nil {
		log.Error("failed to load routes", logger.Fields{
			"error": err.Error(),
		})
	}

	// Create proxy with default configuration
	prx := proxy.New(nil)

	// Create rate limiter
	var rateLimiter *ratelimit.Limiter
	if cfg.RateLimit.Enabled {
		limiter, err := ratelimit.NewLimiter(&cfg.RateLimit)
		if err != nil {
			log.Error("failed to create rate limiter", logger.Fields{
				"error": err.Error(),
			})
		} else {
			rateLimiter = limiter
			log.Info("rate limiter initialized", logger.Fields{
				"backend": cfg.RateLimit.Backend,
			})

			// Register rate limiter health check
			if rateLimiter != nil {
				healthMgr.Register("ratelimit", health.RateLimiterChecker(rateLimiter))
			}
		}
	}

	// Create auth middleware
	var authMw *auth.Middleware
	if cfg.Authorization.Enabled {
		middleware, err := auth.NewMiddleware(&cfg.Authorization)
		if err != nil {
			log.Error("failed to create auth middleware", logger.Fields{
				"error": err.Error(),
			})
		} else {
			authMw = middleware
			log.Info("authorization middleware initialized", logger.Fields{
				"algorithm": cfg.Authorization.JWTSigningAlgorithm,
			})
		}
	}

	return &Server{
		config:        cfg,
		healthManager: healthMgr,
		router:        rtr,
		proxy:         prx,
		rateLimiter:   rateLimiter,
		authMiddleware: authMw,
		logger:        log,
	}
}

// Start starts the server
func (s *Server) Start() error {
	// Create main router
	router := s.setupRouter()

	// Setup HTTP server
	s.httpServer = &http.Server{
		Addr:           fmt.Sprintf(":%d", s.config.Server.HTTPPort),
		Handler:        router,
		ReadTimeout:    s.config.Server.ReadTimeout,
		WriteTimeout:   s.config.Server.WriteTimeout,
		IdleTimeout:    s.config.Server.IdleTimeout,
		MaxHeaderBytes: s.config.Server.MaxHeaderBytes,
	}

	// Setup HTTPS server if TLS is enabled
	if s.config.Server.TLSEnabled {
		tlsConfig := &tls.Config{
			MinVersion:               tls.VersionTLS12,
			PreferServerCipherSuites: true,
			CurvePreferences: []tls.CurveID{
				tls.CurveP256,
				tls.X25519,
			},
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			},
		}

		s.httpsServer = &http.Server{
			Addr:           fmt.Sprintf(":%d", s.config.Server.HTTPSPort),
			Handler:        router,
			ReadTimeout:    s.config.Server.ReadTimeout,
			WriteTimeout:   s.config.Server.WriteTimeout,
			IdleTimeout:    s.config.Server.IdleTimeout,
			MaxHeaderBytes: s.config.Server.MaxHeaderBytes,
			TLSConfig:      tlsConfig,
		}
	}

	// Start servers in goroutines
	errChan := make(chan error, 2)

	// Start HTTP server
	go func() {
		s.logger.Info("starting HTTP server", logger.Fields{
			"port": s.config.Server.HTTPPort,
		})
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("HTTP server error: %w", err)
		}
	}()

	// Start HTTPS server if enabled
	if s.config.Server.TLSEnabled {
		go func() {
			s.logger.Info("starting HTTPS server", logger.Fields{
				"port": s.config.Server.HTTPSPort,
			})
			if err := s.httpsServer.ListenAndServeTLS(
				s.config.Server.TLSCertFile,
				s.config.Server.TLSKeyFile,
			); err != nil && err != http.ErrServerClosed {
				errChan <- fmt.Errorf("HTTPS server error: %w", err)
			}
		}()
	}

	// Setup graceful shutdown
	go s.handleShutdown(errChan)

	// Wait for error or shutdown
	err := <-errChan
	return err
}

// setupRouter sets up the HTTP router with middleware
func (s *Server) setupRouter() http.Handler {
	mux := http.NewServeMux()

	// Health check endpoints
	healthPath := s.config.Observability.HealthPath
	readinessPath := s.config.Observability.ReadinessPath
	livenessPath := s.config.Observability.LivenessPath

	mux.HandleFunc(healthPath, s.healthManager.HealthHandler())
	mux.HandleFunc(readinessPath, s.healthManager.ReadinessHandler())
	mux.HandleFunc(livenessPath, s.healthManager.LivenessHandler())

	// Default handler for all other routes
	mux.HandleFunc("/", s.defaultHandler())

	// Apply middleware chain
	var handler http.Handler = mux

	// Middleware is applied in reverse order (last applied = first executed)
	// Order: Recovery -> CorrelationID -> Logging -> RateLimit -> Auth -> Handler

	// Rate limiting middleware (before auth, after logging)
	if s.rateLimiter != nil {
		handler = ratelimit.Middleware(s.rateLimiter, s.config)(handler)
	}

	// Authorization middleware (after logging, before rate limiting)
	if s.authMiddleware != nil {
		handler = s.authMiddleware.Handler(handler)
	}

	handler = middleware.Logging()(handler)
	handler = middleware.CorrelationID()(handler)
	handler = middleware.Recovery()(handler)

	return handler
}

// defaultHandler returns the default handler for non-health routes
func (s *Server) defaultHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Try to match a route
		match, err := s.router.Match(r)

		correlationID := logger.GetCorrelationID(r.Context())

		if err != nil {
			// No route found
			s.logger.Debug("no route matched", logger.Fields{
				"correlation_id": correlationID,
				"method":         r.Method,
				"path":           r.URL.Path,
			})

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)

			errorResp := map[string]interface{}{
				"error":          "not_found",
				"message":        "No route found for the requested path",
				"correlation_id": correlationID,
				"path":           r.URL.Path,
				"method":         r.Method,
			}

			_ = json.NewEncoder(w).Encode(errorResp)
			return
		}

		// Forward request to backend
		if err := s.proxy.Forward(w, r, match); err != nil {
			s.logger.Error("proxy forward error", logger.Fields{
				"correlation_id": correlationID,
				"error":          err.Error(),
				"backend_url":    match.Route.BackendURL,
			})

			// Check if response was already written
			// If so, we can't write error response
			w.Header().Set("Content-Type", "application/json")

			// Determine appropriate status code based on error
			statusCode := http.StatusBadGateway
			if err.Error() == "circuit breaker open for backend "+match.Route.BackendURL {
				statusCode = http.StatusServiceUnavailable
			}

			w.WriteHeader(statusCode)

			errorResp := map[string]interface{}{
				"error":          "gateway_error",
				"message":        "Failed to forward request to backend service",
				"correlation_id": correlationID,
			}

			_ = json.NewEncoder(w).Encode(errorResp)
		}
	}
}

// handleShutdown handles graceful shutdown
func (s *Server) handleShutdown(errChan chan error) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	sig := <-sigChan
	s.logger.Info("shutdown signal received", logger.Fields{
		"signal": sig.String(),
	})

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), s.config.Server.ShutdownTimeout)
	defer cancel()

	// Shutdown HTTP server
	if s.httpServer != nil {
		s.logger.Info("shutting down HTTP server")
		if err := s.httpServer.Shutdown(ctx); err != nil {
			s.logger.Error("HTTP server shutdown error", logger.Fields{
				"error": err.Error(),
			})
		}
	}

	// Shutdown HTTPS server
	if s.httpsServer != nil {
		s.logger.Info("shutting down HTTPS server")
		if err := s.httpsServer.Shutdown(ctx); err != nil {
			s.logger.Error("HTTPS server shutdown error", logger.Fields{
				"error": err.Error(),
			})
		}
	}

	// Cleanup rate limiter
	if s.rateLimiter != nil {
		s.logger.Info("closing rate limiter")
		if err := s.rateLimiter.Close(); err != nil {
			s.logger.Error("rate limiter close error", logger.Fields{
				"error": err.Error(),
			})
		}
	}

	s.logger.Info("server shutdown complete")
	errChan <- nil
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("initiating server shutdown")

	// Shutdown HTTP server
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown HTTP server: %w", err)
		}
	}

	// Shutdown HTTPS server
	if s.httpsServer != nil {
		if err := s.httpsServer.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown HTTPS server: %w", err)
		}
	}

	// Cleanup rate limiter
	if s.rateLimiter != nil {
		if err := s.rateLimiter.Close(); err != nil {
			return fmt.Errorf("failed to close rate limiter: %w", err)
		}
	}

	return nil
}
