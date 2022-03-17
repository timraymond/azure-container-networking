package nmagent

import (
	"fmt"
	"net/http"
	"time"
)

// Error is a aberrent condition encountered when interacting with the NMAgent
// API
type Error struct {
	Runtime time.Duration // the amount of time the operation has been running
	Limit   time.Duration // the maximum amount of time the operation can run for
	Code    int           // the HTTP status code received
}

func (e Error) Error() string {
	return fmt.Sprintf("nmagent: http status %d", e.Code)
}

// Temporary reports whether the error encountered from NMAgent should be
// considered temporary, and thus retriable
func (e Error) Temporary() bool {
	// We consider Unauthorized responses from NMAgent to be temporary for a
	// certain period of time. This is to allow for situations where an
	// authorization token may not yet be available
	if e.Code == http.StatusUnauthorized {
		return e.Runtime < e.Limit
	}

	// NMAgent will return a 102 (Processing) if the request is taking time to
	// complete. These should be attempted again.
	return e.Code == http.StatusProcessing
}
