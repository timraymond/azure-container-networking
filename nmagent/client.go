package nmagent

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Azure/azure-container-networking/nmagent/internal"
	"github.com/pkg/errors"
)

// NewClient returns an initialized Client using the provided configuration
func NewClient(c Config) (*Client, error) {
	if err := c.Validate(); err != nil {
		return nil, errors.Wrap(err, "validating config")
	}

	client := &Client{
		httpClient: &http.Client{
			Transport: &internal.WireserverTransport{
				Transport: http.DefaultTransport,
			},
		},
		host:      c.Host,
		port:      c.Port,
		enableTLS: c.UseTLS,
		retrier: internal.Retrier{
			// nolint:gomnd // the base parameter is explained in the function
			Cooldown: internal.Exponential(1*time.Second, 2),
		},
	}

	return client, nil
}

// Client is an agent for exchanging information with NMAgent
type Client struct {
	httpClient *http.Client

	// config
	host string
	port uint16

	enableTLS bool

	// unauthorizedGracePeriod is the amount of time Unauthorized responses from
	// NMAgent will be tolerated and retried
	unauthorizedGracePeriod time.Duration

	retrier interface {
		Do(context.Context, func() error) error
	}
}

// Option is a functional option for configuration optional behavior in the
// client
type Option func(*Client)

// WithUnauthorizedGracePeriod is an option to treat Unauthorized (401)
// responses from NMAgent as temporary errors for a configurable amount of time
func WithUnauthorizedGracePeriod(grace time.Duration) Option {
	return func(c *Client) {
		c.unauthorizedGracePeriod = grace
	}
}

// JoinNetwork joins a node to a customer's virtual network
func (c *Client) JoinNetwork(ctx context.Context, jnr JoinNetworkRequest) error {
	req, err := c.buildRequest(ctx, jnr)
	if err != nil {
		return errors.Wrap(err, "building request")
	}

	err = c.retrier.Do(ctx, func() error {
		resp, err := c.httpClient.Do(req) // nolint:govet // the shadow is intentional
		if err != nil {
			return errors.Wrap(err, "executing request")
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return Error{resp.StatusCode}
		}
		return nil
	})

	return err // nolint:wrapcheck // wrapping this just introduces noise
}

// GetNetworkConfiguration retrieves the configuration of a customer's virtual
// network. Only subnets which have been delegated will be returned
func (c *Client) GetNetworkConfiguration(ctx context.Context, gncr GetNetworkConfigRequest) (VirtualNetwork, error) {
	var out VirtualNetwork

	req, err := c.buildRequest(ctx, gncr)
	if err != nil {
		return out, errors.Wrap(err, "building request")
	}

	err = c.retrier.Do(ctx, func() error {
		resp, err := c.httpClient.Do(req) // nolint:govet // the shadow is intentional
		if err != nil {
			return errors.Wrap(err, "executing http request to")
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return Error{resp.StatusCode}
		}

		ct := resp.Header.Get(internal.HeaderContentType)
		if ct != internal.MimeJSON {
			return NewContentError(ct, resp.Body, resp.ContentLength)
		}

		err = json.NewDecoder(resp.Body).Decode(&out)
		if err != nil {
			return errors.Wrap(err, "decoding json response")
		}

		return nil
	})

	return out, err // nolint:wrapcheck // wrapping just introduces noise here
}

// PutNetworkContainer applies a Network Container goal state and publishes it
// to PubSub
func (c *Client) PutNetworkContainer(ctx context.Context, pncr *PutNetworkContainerRequest) error {
	req, err := c.buildRequest(ctx, pncr)
	if err != nil {
		return errors.Wrap(err, "building request")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "submitting request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Error{resp.StatusCode}
	}
	return nil
}

// DeleteNetworkContainer removes a Network Container, its associated IP
// addresses, and network policies from an interface
func (c *Client) DeleteNetworkContainer(ctx context.Context, dcr DeleteContainerRequest) error {
	req, err := c.buildRequest(ctx, dcr)
	if err != nil {
		return errors.Wrap(err, "building request")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "submitting request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Error{resp.StatusCode}
	}

	return nil
}

func (c *Client) hostPort() string {
	port := strconv.Itoa(int(c.port))
	return net.JoinHostPort(c.host, port)
}

func (c *Client) buildRequest(ctx context.Context, req Request) (*http.Request, error) {
	if err := req.Validate(); err != nil {
		return nil, errors.Wrap(err, "validating request")
	}

	fullURL := &url.URL{
		Scheme: c.scheme(),
		Host:   c.hostPort(),
		Path:   req.Path(),
	}

	body, err := req.Body()
	if err != nil {
		return nil, errors.Wrap(err, "retrieving request body")
	}

	// nolint:wrapcheck // wrapping doesn't provide useful information
	return http.NewRequestWithContext(ctx, req.Method(), fullURL.String(), body)
}

func (c *Client) scheme() string {
	if c.enableTLS {
		return "https"
	}
	return "http"
}
