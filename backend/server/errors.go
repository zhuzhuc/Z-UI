package server

import (
	"errors"
	"log"
	"strings"

	"zui/storage"
)

// Version is set via ldflags at build time.
var Version = "dev"

// sanitizeError returns a safe, generic error message for the client.
// The full error is logged server-side for debugging.
func sanitizeError(err error) string {
	if err == nil {
		return ""
	}
	log.Printf("error detail (not sent to client): %v", err)

	if errors.Is(err, storage.ErrNotFound) {
		return "resource not found"
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "unique constraint") || strings.Contains(lower, "unique failed") {
		return "duplicate entry"
	}
	if strings.Contains(lower, "bcrypt") || strings.Contains(lower, "hashedPassword") {
		return "authentication error"
	}
	return "internal error"
}
