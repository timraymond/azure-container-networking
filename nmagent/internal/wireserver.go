package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const (
	_ int64 = 1 << (10 * iota)
	kilobyte
	megabyte
)

const (
	WirePrefix string = "/machine/plugins/?comp=nmagent&type="

	// DefaultBufferSize is the maximum number of bytes read from Wireserver in
	// the event that no Content-Length is provided. The responses are relatively
	// small, so the smallest page size should be sufficient
	DefaultBufferSize int64 = 4 * kilobyte
)

var _ http.RoundTripper = &WireserverTransport{}

// WireserverResponse represents a raw response from Wireserver
type WireserverResponse map[string]json.RawMessage

// StatusCode extracts the embedded HTTP status code from the response from Wireserver
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

// RoundTrip executes arbitrary HTTP requests against Wireserver while applying
// the necessary transformation rules to make such requests acceptable to
// Wireserver
func (w *WireserverTransport) RoundTrip(inReq *http.Request) (*http.Response, error) {
	// RoundTrippers are not allowed to modify the request, so we clone it here.
	// We need to extract the context from the request first since this is _not_
	// cloned. The dependent Wireserver request should have the same deadline and
	// cancellation properties as the inbound request though, hence the reuse.
	ctx := inReq.Context()
	req := inReq.Clone(ctx)

	// the original path of the request must be prefixed with wireserver's path
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

	// wireserver cannot tolerate PUT requests, so it's necessary to transform
	// those to POSTs
	if req.Method == http.MethodPut {
		req.Method = http.MethodPost
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

	if resp.StatusCode != http.StatusOK {
		// something happened at Wireserver, so we should just hand this back up
		return resp, nil
	}

	// at this point we're definitely going to modify the body, so we want to
	// make sure we close the original request's body, since we're going to
	// replace it
	defer func(body io.ReadCloser) {
		body.Close()
	}(resp.Body)

	// buffer the entire response from Wireserver
	clen := resp.ContentLength
	if clen < 0 {
		clen = DefaultBufferSize
	}

	body := make([]byte, clen)
	bodyReader := io.LimitReader(resp.Body, clen)

	numRead, err := io.ReadFull(bodyReader, body)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, fmt.Errorf("reading response from wireserver: %w", err)
	}
	// it's entirely possible at this point that we read less than we allocated,
	// so trim the slice back for decoding
	body = body[:numRead]

	// set the content length properly in case it wasn't set. If it was, this is
	// effectively a no-op
	resp.ContentLength = int64(numRead)

	// it's unclear whether Wireserver sets Content-Type appropriately, so we
	// attempt to decode it first and surface it otherwise
	var wsResp WireserverResponse
	err = json.Unmarshal(body, &wsResp)
	if err != nil {
		// probably not JSON, so figure out what it is, pack it up, and surface it
		// unmodified
		resp.Header.Set(HeaderContentType, http.DetectContentType(body))
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, nil
	}

	// we know that it's JSON now, so communicate that upwards
	resp.Header.Set(HeaderContentType, MimeJSON)

	// set the response status code with the *real* status code
	realCode, err := wsResp.StatusCode()
	if err != nil {
		return resp, fmt.Errorf("retrieving status code from wireserver response: %w", err)
	}

	resp.StatusCode = realCode

	// re-encode the body and re-attach it to the response
	delete(wsResp, "httpStatusCode") // TODO(timraymond): concern of the response

	outBody, err := json.Marshal(wsResp)
	if err != nil {
		return resp, fmt.Errorf("re-encoding json response from wireserver: %w", err)
	}

	resp.Body = io.NopCloser(bytes.NewReader(outBody))

	return resp, nil
}
