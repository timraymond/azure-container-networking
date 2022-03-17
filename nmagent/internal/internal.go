package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const WirePrefix string = "/machine/plugins/?comp=nmagent&type="

var _ http.RoundTripper = &WireserverTransport{}

// WireserverResponse represents a raw response from Wireserver
type WireserverResponse map[string]json.RawMessage

func (w WireserverResponse) StatusCode() (int, error) {
	if status, ok := w["httpStatusCode"]; ok {
		var statusStr string
		err := json.Unmarshal(status, &statusStr)
		if err != nil {
			return 0, fmt.Errorf("unmarshaling httpStatusCode from Wireserver: %w", err)
		}

		if code, err := strconv.Atoi(statusStr); err != nil {
			return code, fmt.Errorf("parsing http status code from wireserver: %w", err)
		} else {
			return code, nil
		}
	}
	return 0, fmt.Errorf("no httpStatusCode property returned in Wireserver response")
}

// WireserverTransport is an http.RoundTripper that applies transformation
// rules to inbound requests necessary to make them compatible with Wireserver
type WireserverTransport struct {
	Transport http.RoundTripper
}

func (w *WireserverTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// the original path of the request must be prefixed with wireserver's path
	origPath := req.URL.Path
	path := WirePrefix
	if req.URL.Path != "" {
		path += req.URL.Path[1:]
	}

	// the query string from the request must have its constituent parts (?,=,&)
	// transformed to slashes and appended to the query
	if req.URL.RawQuery != "" {
		query := req.URL.RawQuery
		query = strings.ReplaceAll(query, "?", "/")
		query = strings.ReplaceAll(query, "=", "/")
		query = strings.ReplaceAll(query, "&", "/")
		path += "/" + query
	}

	req.URL.Path = path
	// ensure that nothing has changed from the caller's perspective by resetting
	// the URL
	defer func() {
		req.URL.Path = origPath
	}()

	// wireserver cannot tolerate PUT requests, so it's necessary to transform those to POSTs
	if req.Method == http.MethodPut {
		req.Method = http.MethodPost
		defer func() {
			req.Method = http.MethodPut
		}()
	}

	// all POST requests (and by extension, PUT) must have a non-nil body
	if req.Method == http.MethodPost && req.Body == nil {
		req.Body = io.NopCloser(strings.NewReader(""))
	}

	// execute the request to the downstream transport
	resp, err := w.Transport.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	// we want to close this because we're going to replace it
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return resp, nil
	}

	// correct the HTTP status returned from wireserver
	var wsResp WireserverResponse
	err = json.NewDecoder(resp.Body).Decode(&wsResp)
	if err != nil {
		return resp, fmt.Errorf("decoding json response from wireserver: %w", err)
	}

	// set the response status code with the *real* status code
	realCode, err := wsResp.StatusCode()
	if err != nil {
		return resp, fmt.Errorf("retrieving status code from wireserver response: %w", err)
	}

	resp.StatusCode = realCode

	// re-encode the body and re-attach it to the response
	delete(wsResp, "httpStatusCode") // TODO(timraymond): concern of the response

	body, err := json.Marshal(wsResp)
	if err != nil {
		return resp, fmt.Errorf("re-encoding json response from wireserver: %w", err)
	}

	resp.Body = io.NopCloser(bytes.NewReader(body))

	return resp, nil
}

type TemporaryError interface {
	error
	Temporary() bool
}

// BackoffRetry implements cancellable exponential backoff of some arbitrary
// function
func BackoffRetry(ctx context.Context, run func() error) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := run()
		if err != nil {
			// check to see if it's temporary
			var tempErr TemporaryError
			if ok := errors.As(err, &tempErr); ok && tempErr.Temporary() {
				continue
			}

			// since it's not temporary, it can't be retried, so...
			return err
		}
		return nil
	}
}
