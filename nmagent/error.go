package nmagent

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	pkgerrors "github.com/pkg/errors"

	"github.com/Azure/azure-container-networking/nmagent/internal"
)

// ContentError is encountered when an unexpected content type is obtained from
// NMAgent
type ContentError struct {
	Type string // the mime type of the content received
	Body []byte // the received body
}

func (c ContentError) Error() string {
	if c.Type == internal.MimeOctetStream {
		return fmt.Sprintf("unexpected content type %q: body length: %d", c.Type, len(c.Body))
	}
	return fmt.Sprintf("unexpected content type %q: body: %s", c.Type, c.Body)
}

// NewContentError creates a ContentError from a provided reader and limit.
func NewContentError(contentType string, in io.Reader, limit int64) error {
	out := ContentError{
		Type: contentType,
		Body: make([]byte, limit),
	}

	bodyReader := io.LimitReader(in, limit)

	read, err := io.ReadFull(bodyReader, out.Body)
	earlyEOF := errors.Is(err, io.ErrUnexpectedEOF)
	if err != nil && !earlyEOF {
		return pkgerrors.Wrap(err, "reading unexpected content body")
	}

	if earlyEOF {
		out.Body = out.Body[:read]
	}

	return out
}

// Error is a aberrent condition encountered when interacting with the NMAgent
// API
type Error struct {
	Runtime time.Duration // the amount of time the operation has been running
	Limit   time.Duration // the maximum amount of time the operation can run for
	Code    int           // the HTTP status code received
}

// Error constructs a string representation of this error in accordance with
// the error interface
func (e Error) Error() string {
	return fmt.Sprintf("nmagent: http status %d: %s", e.Code, e.Message())
}

// Message interprets the HTTP Status code from NMAgent and returns the
// corresponding explanation from the documentation
func (e Error) Message() string {
	switch e.Code {
	case http.StatusProcessing:
		return "the request is taking time to process. the caller should try the request again"
	case http.StatusUnauthorized:
		return "the request did not originate from an interface with an OwningServiceInstanceId property"
	case http.StatusInternalServerError:
		return "error occurred during nmagent's request processing"
	default:
		return "undocumented nmagent error"
	}
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
