package tracing

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

// Middleware creates a tracing middleware that extracts and propagates trace context
func Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract trace context from incoming request headers
			ctx := extractTraceContext(r)

			// Start a new span for this request
			spanName := r.Method + " " + r.URL.Path
			ctx, span := Tracer().Start(
				ctx,
				spanName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					semconv.HTTPMethodKey.String(r.Method),
					semconv.HTTPURLKey.String(r.URL.String()),
					semconv.HTTPTargetKey.String(r.URL.Path),
					semconv.HTTPSchemeKey.String(scheme(r)),
					semconv.HTTPHostKey.String(r.Host),
					semconv.HTTPUserAgentKey.String(r.UserAgent()),
					semconv.HTTPClientIPKey.String(clientIP(r)),
				),
			)
			defer span.End()

			// Trace context is now available in ctx for downstream handlers
			// Trace ID and Span ID can be extracted from the span context if needed

			// Wrap response writer to capture status code
			wrapped := &statusRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Call next handler with traced context
			next.ServeHTTP(wrapped, r.WithContext(ctx))

			// Record response status
			span.SetAttributes(semconv.HTTPStatusCodeKey.Int(wrapped.statusCode))

			// Set span status based on HTTP status code
			if wrapped.statusCode >= 400 && wrapped.statusCode < 500 {
				span.SetStatus(codes.Error, http.StatusText(wrapped.statusCode))
			} else if wrapped.statusCode >= 500 {
				span.SetStatus(codes.Error, http.StatusText(wrapped.statusCode))
			} else {
				span.SetStatus(codes.Ok, "")
			}
		})
	}
}

// extractTraceContext extracts trace context from HTTP headers
func extractTraceContext(r *http.Request) context.Context {
	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)

	return propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
}

// InjectTraceContext injects trace context into HTTP headers for outgoing requests
func InjectTraceContext(ctx context.Context, req *http.Request) {
	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)

	propagator.Inject(ctx, propagation.HeaderCarrier(req.Header))
}

// scheme returns the request scheme (http or https)
func scheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

// clientIP extracts the client IP from the request
func clientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		return forwarded
	}

	// Check X-Real-IP header
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}

// statusRecorder wraps http.ResponseWriter to record status code
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code
func (rec *statusRecorder) WriteHeader(code int) {
	rec.statusCode = code
	rec.ResponseWriter.WriteHeader(code)
}

// Write ensures status code is recorded even if WriteHeader is not called
func (rec *statusRecorder) Write(b []byte) (int, error) {
	return rec.ResponseWriter.Write(b)
}

// StartSpanFromRequest starts a new span for an operation within a request
func StartSpanFromRequest(r *http.Request, operationName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return Tracer().Start(r.Context(), operationName, opts...)
}

// AddEventToSpan adds an event to the current span in the context
func AddEventToSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := SpanFromContext(ctx)
	if span != nil && span.IsRecording() {
		span.AddEvent(name, trace.WithAttributes(attrs...))
	}
}
