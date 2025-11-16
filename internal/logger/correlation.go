package logger

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// GenerateCorrelationID generates a new correlation ID
// Returns a UUID v4 format correlation ID
func GenerateCorrelationID() string {
	uuid := make([]byte, 16)
	_, err := rand.Read(uuid)
	if err != nil {
		// Fallback to a less random but still unique ID
		return fmt.Sprintf("fallback-%d", randomInt63())
	}

	// Set version (4) and variant bits
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // Version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // Variant is 10

	// Format as UUID string
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4],
		uuid[4:6],
		uuid[6:8],
		uuid[8:10],
		uuid[10:16],
	)
}

// GenerateShortID generates a shorter correlation ID (16 characters)
func GenerateShortID() string {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		return fmt.Sprintf("%016x", randomInt63())
	}
	return hex.EncodeToString(b)
}

// randomInt63 provides a fallback random number generator
func randomInt63() int64 {
	// Simple fallback using current time
	return int64(^uint64(0) >> 1)
}
