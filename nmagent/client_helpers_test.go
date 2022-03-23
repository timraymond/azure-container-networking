package nmagent

import (
	"dnc/nmagent/internal"
	"net/http"
)

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
