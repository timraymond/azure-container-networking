package nmagent

import (
	"dnc/nmagent/internal"
	"net/http"
)

// NewTestClient creates an NMAgent Client suitable for use in tests. This is
// unavailable in production builds
func NewTestClient(tripper http.RoundTripper) *Client {
	return &Client{
		httpClient: &http.Client{
			Transport: &internal.WireserverTransport{
				Transport: tripper,
			},
		},
	}
}
