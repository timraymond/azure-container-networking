package nmagent

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"dnc/nmagent/internal"
)

// NewClient returns an initialized Client using the provided configuration
func NewClient(host string, port uint16, opts ...Option) *Client {
	client := &Client{
		httpClient: &http.Client{
			Transport: &internal.WireserverTransport{
				Transport: http.DefaultTransport,
			},
		},
		host: host,
		port: port,
		retrier: internal.Retrier{
			Cooldown: internal.Exponential(1*time.Second, 2),
		},
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

// Client is an agent for exchanging information with NMAgent
type Client struct {
	httpClient *http.Client

	// config
	host string
	port uint16

	disableTLS bool

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

// InsecureDisableTLS is an option to disable TLS communications with NMAgent
func InsecureDisableTLS() Option {
	return func(c *Client) {
		c.disableTLS = true
	}
}

// WithUnauthorizedGracePeriod is an option to treat Unauthorized (401)
// responses from NMAgent as temporary errors for a configurable amount of time
func WithUnauthorizedGracePeriod(grace time.Duration) Option {
	return func(c *Client) {
		c.unauthorizedGracePeriod = grace
	}
}

// JoinNetwork joins a node to a customer's virtual network
func (c *Client) JoinNetwork(ctx context.Context, jnr JoinNetworkRequest) error {
	requestStart := time.Now()

	req, err := c.buildRequest(ctx, jnr)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}

	err = c.retrier.Do(ctx, func() error {
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("executing request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return c.error(time.Since(requestStart), resp.StatusCode)
		}
		return nil
	})

	return err
}

// GetNetworkConfiguration retrieves the configuration of a customer's virtual
// network. Only subnets which have been delegated will be returned
func (c *Client) GetNetworkConfiguration(ctx context.Context, gncr GetNetworkConfigRequest) (VirtualNetwork, error) {
	requestStart := time.Now()

	var out VirtualNetwork

	req, err := c.buildRequest(ctx, gncr)
	if err != nil {
		return out, fmt.Errorf("building request: %w", err)
	}

	err = c.retrier.Do(ctx, func() error {
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("executing http request to: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return c.error(time.Since(requestStart), resp.StatusCode)
		}

		err = json.NewDecoder(resp.Body).Decode(&out)
		if err != nil {
			return fmt.Errorf("decoding json response: %w", err)
		}

		return nil
	})

	return out, err
}

// PutNetworkContainer applies a Network Container goal state and publishes it
// to PubSub
func (c *Client) PutNetworkContainer(ctx context.Context, pncr PutNetworkContainerRequest) error {
	requestStart := time.Now()

	req, err := c.buildRequest(ctx, pncr)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("submitting request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.error(time.Since(requestStart), resp.StatusCode)
	}
	return nil
}

// DeleteNetworkContainer removes a Network Container, its associated IP
// addresses, and network policies from an interface
func (c *Client) DeleteNetworkContainer(ctx context.Context, dcr DeleteContainerRequest) error {
	requestStart := time.Now()

	req, err := c.buildRequest(ctx, dcr)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("submitting request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.error(time.Since(requestStart), resp.StatusCode)
	}

	return nil
}

func (c *Client) hostPort() string {
	port := strconv.Itoa(int(c.port))
	return net.JoinHostPort(c.host, port)
}

func (c *Client) buildRequest(ctx context.Context, req Request) (*http.Request, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validating request: %w", err)
	}

	fullURL := &url.URL{
		Scheme: c.scheme(),
		Host:   c.hostPort(),
		Path:   req.Path(),
	}

	body, err := req.Body()
	if err != nil {
		return nil, fmt.Errorf("retrieving request body: %w", err)
	}

	return http.NewRequestWithContext(ctx, req.Method(), fullURL.String(), body)
}

func (c *Client) scheme() string {
	if c.disableTLS {
		return "http"
	}
	return "https"
}

// error constructs a NMAgent error while providing some information configured
// at instantiation
func (c *Client) error(runtime time.Duration, code int) error {
	return Error{
		Runtime: runtime,
		Limit:   c.unauthorizedGracePeriod,
		Code:    code,
	}
}
