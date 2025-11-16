package tracing

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/maltehedderich/api-gateway-go/internal/logger"
)

const (
	// TracerName is the name of the tracer used throughout the application
	TracerName = "github.com/maltehedderich/api-gateway-go"
)

var (
	// tracerProvider is the global tracer provider
	tracerProvider *sdktrace.TracerProvider
	// log is the logger for tracing operations
	log *logger.ComponentLogger
)

// Config contains tracing configuration
type Config struct {
	// Enabled determines if tracing is enabled
	Enabled bool
	// Endpoint is the OTLP collector endpoint (e.g., http://localhost:4318)
	Endpoint string
	// ServiceName is the name of the service
	ServiceName string
	// ServiceVersion is the version of the service
	ServiceVersion string
	// Environment is the deployment environment (dev, staging, prod)
	Environment string
	// SampleRate is the fraction of traces to sample (0.0 to 1.0)
	SampleRate float64
}

// Init initializes the distributed tracing system
func Init(cfg *Config) error {
	log = logger.Get().WithComponent("tracing")

	if !cfg.Enabled {
		log.Info("distributed tracing is disabled")
		// Set up a no-op tracer provider
		otel.SetTracerProvider(noop.NewTracerProvider())
		return nil
	}

	// Create resource with service information
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
			semconv.ServiceVersionKey.String(cfg.ServiceVersion),
			semconv.DeploymentEnvironmentKey.String(cfg.Environment),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	// Create OTLP HTTP exporter
	client := otlptracehttp.NewClient(
		otlptracehttp.WithEndpoint(cfg.Endpoint),
		otlptracehttp.WithInsecure(), // Use WithTLSClientConfig for secure connections in production
	)

	exporter, err := otlptrace.New(context.Background(), client)
	if err != nil {
		return fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create tracer provider with sampler
	sampler := sdktrace.ParentBased(
		sdktrace.TraceIDRatioBased(cfg.SampleRate),
	)

	tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sampler),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tracerProvider)

	// Set global propagator to support W3C Trace Context
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	log.Info("distributed tracing initialized", logger.Fields{
		"endpoint":      cfg.Endpoint,
		"service_name":  cfg.ServiceName,
		"environment":   cfg.Environment,
		"sample_rate":   cfg.SampleRate,
	})

	return nil
}

// Shutdown gracefully shuts down the tracer provider
func Shutdown(ctx context.Context) error {
	if tracerProvider == nil {
		return nil
	}

	log.Info("shutting down tracing")

	// Create a timeout context for shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := tracerProvider.Shutdown(shutdownCtx); err != nil {
		log.Error("failed to shutdown tracer provider", logger.Fields{
			"error": err.Error(),
		})
		return err
	}

	log.Info("tracing shutdown complete")
	return nil
}

// Tracer returns a tracer instance
func Tracer() trace.Tracer {
	return otel.Tracer(TracerName)
}

// SpanFromContext returns the current span from the context
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// ContextWithSpan returns a new context with the given span
func ContextWithSpan(ctx context.Context, span trace.Span) context.Context {
	return trace.ContextWithSpan(ctx, span)
}

// StartSpan starts a new span with the given name and options
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return Tracer().Start(ctx, name, opts...)
}

// RecordError records an error on the current span
func RecordError(ctx context.Context, err error) {
	span := SpanFromContext(ctx)
	if span != nil && err != nil {
		span.RecordError(err)
	}
}

// TraceID returns the trace ID from the context
func TraceID(ctx context.Context) string {
	span := SpanFromContext(ctx)
	if span != nil && span.SpanContext().IsValid() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// SpanID returns the span ID from the context
func SpanID(ctx context.Context) string {
	span := SpanFromContext(ctx)
	if span != nil && span.SpanContext().IsValid() {
		return span.SpanContext().SpanID().String()
	}
	return ""
}
