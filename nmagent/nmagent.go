package nmagent

import (
	"fmt"
	"net/http"
)

// Error is a aberrent condition encountered when interacting with the NMAgent
// API
type Error struct {
	Code int // the HTTP status code received
}

func (e Error) Error() string {
	return fmt.Sprintf("nmagent: http status %d", e.Code)
}

// Temporary reports whether the error encountered from NMAgent should be
// considered temporary, and thus retriable
func (e Error) Temporary() bool {
	// NMAgent will return a 102 (Processing) if the request is taking time to
	// complete. These should be attempted again. As such, it's the only
	// retriable status code
	return e.Code == http.StatusProcessing
}
