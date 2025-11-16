package tracing

import (
	"context"
	"os"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/maltehedderich/api-gateway-go/internal/logger"
)

func init() {
	// Initialize logger for tests
	logger.Init(logger.InfoLevel, "json", os.Stdout)
}

func TestInit_Disabled(t *testing.T) {
	cfg := &Config{
		Enabled: false,
	}

	err := Init(cfg)
	if err != nil {
		t.Fatalf("Init with disabled tracing should not error: %v", err)
	}

	// Tracer should be available even when disabled (no-op)
	tracer := Tracer()
	if tracer == nil {
		t.Fatal("Tracer should not be nil even when disabled")
	}

	// Should be able to create a span (will be no-op)
	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	if ctx == nil {
		t.Fatal("Context should not be nil")
	}
}

func TestInit_Enabled(t *testing.T) {
	// Skip if no test endpoint is available
	t.Skip("Skipping integration test - requires OTLP endpoint")

	cfg := &Config{
		Enabled:        true,
		Endpoint:       "localhost:4318",
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		SampleRate:     1.0,
	}

	err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Clean up
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := Shutdown(ctx); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

func TestStartSpan(t *testing.T) {
	// Initialize with disabled tracing for testing
	cfg := &Config{
		Enabled: false,
	}
	_ = Init(cfg)

	ctx := context.Background()
	newCtx, span := StartSpan(ctx, "test-operation")
	defer span.End()

	if newCtx == nil {
		t.Fatal("Context should not be nil")
	}

	if span == nil {
		t.Fatal("Span should not be nil")
	}
}

func TestSpanFromContext(t *testing.T) {
	cfg := &Config{
		Enabled: false,
	}
	_ = Init(cfg)

	ctx := context.Background()
	ctx, span := StartSpan(ctx, "test-operation")
	defer span.End()

	retrievedSpan := SpanFromContext(ctx)
	if retrievedSpan == nil {
		t.Fatal("Should be able to retrieve span from context")
	}
}

func TestTraceID(t *testing.T) {
	cfg := &Config{
		Enabled: false,
	}
	_ = Init(cfg)

	ctx := context.Background()
	ctx, span := StartSpan(ctx, "test-operation")
	defer span.End()

	// For no-op tracer, trace ID will be empty
	traceID := TraceID(ctx)
	// No-op spans don't have valid trace IDs
	if traceID != "" {
		t.Logf("Trace ID: %s", traceID)
	}
}

func TestSpanID(t *testing.T) {
	cfg := &Config{
		Enabled: false,
	}
	_ = Init(cfg)

	ctx := context.Background()
	ctx, span := StartSpan(ctx, "test-operation")
	defer span.End()

	// For no-op tracer, span ID will be empty
	spanID := SpanID(ctx)
	// No-op spans don't have valid span IDs
	if spanID != "" {
		t.Logf("Span ID: %s", spanID)
	}
}

func TestRecordError(t *testing.T) {
	cfg := &Config{
		Enabled: false,
	}
	_ = Init(cfg)

	ctx := context.Background()
	ctx, span := StartSpan(ctx, "test-operation")
	defer span.End()

	// Should not panic even with no-op tracer
	testErr := &testError{msg: "test error"}
	RecordError(ctx, testErr)
}

func TestRecordError_NilError(t *testing.T) {
	cfg := &Config{
		Enabled: false,
	}
	_ = Init(cfg)

	ctx := context.Background()

	// Should not panic with nil error
	RecordError(ctx, nil)
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestShutdown_NotInitialized(t *testing.T) {
	// Reset tracerProvider
	tracerProvider = nil

	ctx := context.Background()
	err := Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown should not error when not initialized: %v", err)
	}
}

func TestContextWithSpan(t *testing.T) {
	cfg := &Config{
		Enabled: false,
	}
	_ = Init(cfg)

	ctx := context.Background()
	_, span := StartSpan(ctx, "test-operation")

	newCtx := ContextWithSpan(ctx, span)
	if newCtx == nil {
		t.Fatal("Context should not be nil")
	}

	retrievedSpan := trace.SpanFromContext(newCtx)
	if retrievedSpan == nil {
		t.Fatal("Should be able to retrieve span from context")
	}
}
