package nmagent

import (
	"net/http"

	"github.com/Azure/azure-container-networking/nmagent/internal"
)

// Note: this file exists to add two additional options with access to the
// internals of Client. It's a *_test file so that it is only compiled during
// tests.

// WithTransport allows a test to specify a particular http.RoundTripper for
// use in testing scenarios
func WithTransport(tripper http.RoundTripper) Option {
	return func(c *Client) {
		c.httpClient = &http.Client{
			Transport: &internal.WireserverTransport{
				Transport: tripper,
			},
		}
	}
}

// NoBackoff disables exponential backoff in the client
func NoBackoff() Option {
	return func(c *Client) {
		c.retrier = internal.Retrier{
			Cooldown: internal.AsFastAsPossible(),
		}
	}
}
