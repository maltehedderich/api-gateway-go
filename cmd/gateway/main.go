package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/maltehedderich/api-gateway-go/internal/config"
	"github.com/maltehedderich/api-gateway-go/internal/health"
	"github.com/maltehedderich/api-gateway-go/internal/logger"
	"github.com/maltehedderich/api-gateway-go/internal/metrics"
	"github.com/maltehedderich/api-gateway-go/internal/server"
	"github.com/maltehedderich/api-gateway-go/internal/tracing"
)

var (
	configFile = flag.String("config", "", "Path to configuration file")
	version    = "1.0.0"
	buildTime  = "unknown"
	gitCommit  = "unknown"
)

func main() {
	flag.Parse()

	// Print version info
	fmt.Printf("API Gateway v%s (commit: %s, built: %s)\n", version, gitCommit, buildTime)

	// Load configuration
	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logLevel, err := logger.ParseLevel(cfg.Logging.Level)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid log level: %v\n", err)
		os.Exit(1)
	}

	var logOutput *os.File
	var logFileCloser func()
	switch cfg.Logging.Output {
	case "stdout":
		logOutput = os.Stdout
	case "stderr":
		logOutput = os.Stderr
	default:
		// File output
		logOutput, err = os.OpenFile(cfg.Logging.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
			os.Exit(1)
		}
		logFileCloser = func() {
			if err := logOutput.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to close log file: %v\n", err)
			}
		}
		defer logFileCloser()
	}

	logger.Init(logLevel, cfg.Logging.Format, logOutput)

	// Get logger
	log := logger.Get().WithComponent("main")
	log.Info("starting API gateway", logger.Fields{
		"version":    version,
		"git_commit": gitCommit,
		"build_time": buildTime,
	})

	// Set up sanitize patterns if configured
	if len(cfg.Logging.SanitizePatterns) > 0 {
		if err := logger.Get().SetSanitizePatterns(cfg.Logging.SanitizePatterns); err != nil {
			log.Error("failed to set sanitize patterns", logger.Fields{
				"error": err.Error(),
			})
			os.Exit(1)
		}
	}

	// Set component-specific log levels if configured
	for component, levelStr := range cfg.Logging.ComponentLevels {
		level, err := logger.ParseLevel(levelStr)
		if err != nil {
			log.Warn("invalid component log level", logger.Fields{
				"component": component,
				"level":     levelStr,
				"error":     err.Error(),
			})
			continue
		}
		logger.Get().SetComponentLevel(component, level)
	}

	// Initialize metrics if enabled
	if cfg.Observability.MetricsEnabled {
		metrics.Init()
		log.Info("metrics initialized", logger.Fields{
			"metrics_path": cfg.Observability.MetricsPath,
		})
	}

	// Initialize distributed tracing if enabled
	if cfg.Observability.TracingEnabled {
		tracingConfig := &tracing.Config{
			Enabled:        cfg.Observability.TracingEnabled,
			Endpoint:       cfg.Observability.TracingEndpoint,
			ServiceName:    "api-gateway",
			ServiceVersion: version,
			Environment:    getEnvironment(cfg),
			SampleRate:     1.0, // Sample all traces by default
		}

		if err := tracing.Init(tracingConfig); err != nil {
			log.Error("failed to initialize tracing", logger.Fields{
				"error": err.Error(),
			})
			// Continue without tracing - don't fail startup
		} else {
			log.Info("distributed tracing initialized", logger.Fields{
				"endpoint": cfg.Observability.TracingEndpoint,
			})
		}
	}

	// Initialize health check manager
	healthMgr := health.NewManager()

	// Register config health check
	healthMgr.Register("config", health.ConfigChecker(func() bool {
		return config.Get() != nil
	}))

	// Create and start server
	srv := server.New(cfg, healthMgr)

	log.Info("configuration loaded successfully", logger.Fields{
		"http_port":  cfg.Server.HTTPPort,
		"https_port": cfg.Server.HTTPSPort,
		"tls_enabled": cfg.Server.TLSEnabled,
	})

	// Start server (blocks until shutdown)
	if err := srv.Start(); err != nil {
		log.Error("server error", logger.Fields{
			"error": err.Error(),
		})
		os.Exit(1)
	}

	log.Info("API gateway stopped")
}

// getEnvironment determines the deployment environment
func getEnvironment(cfg *config.Config) string {
	// Try to determine environment from configuration or environment variables
	if env := os.Getenv("ENVIRONMENT"); env != "" {
		return env
	}
	if env := os.Getenv("ENV"); env != "" {
		return env
	}

	// Default to development
	return "development"
}

