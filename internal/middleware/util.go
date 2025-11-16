package middleware

import (
	"bufio"
	"encoding/json"
	"net"
	"net/http"
	"strings"
)

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
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

// WriteJSON writes a JSON response
func WriteJSON(w http.ResponseWriter, v interface{}) error {
	return json.NewEncoder(w).Encode(v)
}

// writeJSON is an internal helper
func writeJSON(w http.ResponseWriter, v interface{}) error {
	return WriteJSON(w, v)
}

// ResponseWriter wraps http.ResponseWriter to capture status code and response size
type ResponseWriter struct {
	http.ResponseWriter
	status      int
	size        int
	wroteHeader bool
}

// NewResponseWriter creates a new ResponseWriter
func NewResponseWriter(w http.ResponseWriter) *ResponseWriter {
	return &ResponseWriter{
		ResponseWriter: w,
		status:         http.StatusOK, // default status
	}
}

// WriteHeader captures the status code
func (rw *ResponseWriter) WriteHeader(statusCode int) {
	if !rw.wroteHeader {
		rw.status = statusCode
		rw.wroteHeader = true
		rw.ResponseWriter.WriteHeader(statusCode)
	}
}

// Write captures the response size
func (rw *ResponseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.size += n
	return n, err
}

// Status returns the status code
func (rw *ResponseWriter) Status() int {
	return rw.status
}

// Size returns the response size
func (rw *ResponseWriter) Size() int {
	return rw.size
}

// Flush implements http.Flusher
func (rw *ResponseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack implements http.Hijacker
func (rw *ResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}
