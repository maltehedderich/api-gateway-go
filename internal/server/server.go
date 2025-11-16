package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/maltehedderich/api-gateway-go/internal/config"
	"github.com/maltehedderich/api-gateway-go/internal/health"
	"github.com/maltehedderich/api-gateway-go/internal/logger"
	"github.com/maltehedderich/api-gateway-go/internal/middleware"
)

// Server represents the API Gateway server
type Server struct {
	config        *config.Config
	httpServer    *http.Server
	httpsServer   *http.Server
	healthManager *health.Manager
	logger        *logger.ComponentLogger
}

// New creates a new server instance
func New(cfg *config.Config, healthMgr *health.Manager) *Server {
	return &Server{
		config:        cfg,
		healthManager: healthMgr,
		logger:        logger.Get().WithComponent("server"),
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
	// Order: Recovery -> CorrelationID -> Logging -> Handler

	handler = middleware.Logging()(handler)
	handler = middleware.CorrelationID()(handler)
	handler = middleware.Recovery()(handler)

	return handler
}

// defaultHandler returns the default handler for non-health routes
func (s *Server) defaultHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// For now, just return a simple response
		// This will be replaced with actual routing in Phase 2
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := map[string]interface{}{
			"message": "API Gateway is running",
			"version": "1.0.0",
			"path":    r.URL.Path,
		}

		correlationID := logger.GetCorrelationID(r.Context())
		if correlationID != "" {
			response["correlation_id"] = correlationID
		}

		_ = middleware.WriteJSON(w, response)
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

	return nil
}
