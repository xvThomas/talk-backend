package usage

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// generateTraceID generates a random 16-byte trace ID as a hex string
func generateTraceID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// generateSpanID generates a random 8-byte span ID as a hex string
func generateSpanID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// generateTraceIDWithTime generates a trace ID with embedded timestamp for better tracing
func generateTraceIDWithTime() string {
	// Use current timestamp (8 bytes) + random (8 bytes) for trace ID
	timestamp := time.Now().Unix()
	timestampBytes := make([]byte, 8)
	for i := 0; i < 8; i++ {
		timestampBytes[i] = byte(timestamp >> (8 * (7 - i)))
	}

	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)

	traceBytes := append(timestampBytes, randomBytes...)
	return hex.EncodeToString(traceBytes)
}
