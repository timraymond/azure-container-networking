package nmagent

import (
	"bytes"
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

const (
	PutNCRequestPath string = "/NetworkManagement/interfaces/%s/networkContainers/%s/authenticationToken/%s/api-version/1"
)

// NewClient returns an initialized Client using the provided configuration
func NewClient(host string, port uint16, grace time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{
			Transport: &internal.WireserverTransport{
				Transport: http.DefaultTransport,
			},
		},
		Host:                    host,
		Port:                    port,
		UnauthorizedGracePeriod: grace,
		Retrier: internal.Retrier{
			Cooldown: internal.Exponential(1*time.Second, 2*time.Second),
		},
	}
}

// Client is an agent for exchanging information with NMAgent
type Client struct {
	httpClient *http.Client

	// config
	Host string
	Port uint16

	// UnauthorizedGracePeriod is the amount of time Unauthorized responses from
	// NMAgent will be tolerated and retried
	UnauthorizedGracePeriod time.Duration

	Retrier interface {
		Do(context.Context, func() error) error
	}
}

// JoinNetwork joins a node to a customer's virtual network
func (c *Client) JoinNetwork(ctx context.Context, jnr JoinNetworkRequest) error {
	requestStart := time.Now()

	if err := jnr.Validate(); err != nil {
		return fmt.Errorf("validating join network request: %w", err)
	}

	joinURL := &url.URL{
		Scheme: "https",
		Host:   c.hostPort(),
		Path:   jnr.Path(),
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, joinURL.String(), nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	err = c.Retrier.Do(ctx, func() error {
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

	if err := gncr.Validate(); err != nil {
		return out, fmt.Errorf("validating request: %w", err)
	}

	path := &url.URL{
		Scheme: "https",
		Host:   c.hostPort(),
		Path:   gncr.Path(),
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path.String(), nil)
	if err != nil {
		return out, fmt.Errorf("creating http request to %q: %w", path.String(), err)
	}

	err = c.Retrier.Do(ctx, func() error {
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("executing http request to %q: %w", path.String(), err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return c.error(time.Since(requestStart), resp.StatusCode)
		}

		err = json.NewDecoder(resp.Body).Decode(&out)
		if err != nil {
			return fmt.Errorf("decoding json response for %q: %w", path.String(), err)
		}

		return nil
	})

	if err != nil {
		// no need to wrap, as the retry wrapper is intended to be transparent
		return out, err
	}
	return out, nil
}

// PutNetworkContainer applies a Network Container goal state and publishes it
// to PubSub
func (c *Client) PutNetworkContainer(ctx context.Context, nc NetworkContainerRequest) error {
	requestStart := time.Now()

	path := &url.URL{
		Scheme: "https",
		Host:   c.hostPort(),
		Path:   fmt.Sprintf(PutNCRequestPath, nc.PrimaryAddress, nc.ID, nc.AuthenticationToken),
	}

	body, err := json.Marshal(nc)
	if err != nil {
		return fmt.Errorf("encoding request as JSON: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, path.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
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

	if err := dcr.Validate(); err != nil {
		return fmt.Errorf("validating request: %w", err)
	}

	path := &url.URL{
		Scheme: "https",
		Host:   c.hostPort(),
		Path:   dcr.Path(),
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, path.String(), nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
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
	port := strconv.Itoa(int(c.Port))
	return net.JoinHostPort(c.Host, port)
}

// error constructs a NMAgent error while providing some information configured
// at instantiation
func (c *Client) error(runtime time.Duration, code int) error {
	return Error{
		Runtime: runtime,
		Limit:   c.UnauthorizedGracePeriod,
		Code:    code,
	}
}
