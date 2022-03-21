package nmagent

import (
	"bytes"
	"context"
	"dnc/nmagent/internal"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
)

const (
	JoinNetworkPath      string = "/NetworkManagement/joinedVirtualNetworks/%s/api-version/1"
	GetNetworkConfigPath string = "/NetworkManagement/joinedVirtualNetworks/%s/api-version/1"
	PutNCRequestPath     string = "/NetworkManagement/interfaces/%s/networkContainers/%s/authenticationToken/%s/api-version/1"
)

// NewClient returns an initialized Client using the provided configuration
func NewClient(host, port string, grace time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{
			Transport: &internal.WireserverTransport{
				Transport: http.DefaultTransport,
			},
		},
		Host:                    host,
		Port:                    port,
		UnauthorizedGracePeriod: grace,
	}
}

// Client is an agent for exchanging information with NMAgent
type Client struct {
	httpClient *http.Client

	// config
	Host string
	Port string

	// UnauthorizedGracePeriod is the amount of time Unauthorized responses from
	// NMAgent will be tolerated and retried
	UnauthorizedGracePeriod time.Duration
}

// JoinNetwork joins a node to a customer's virtual network
func (c *Client) JoinNetwork(ctx context.Context, networkID string) error {
	requestStart := time.Now()

	// we need to be a little defensive, because there is no bad request response
	// from NMAgent
	if _, err := uuid.Parse(networkID); err != nil {
		return fmt.Errorf("bad network ID %q: %w", networkID, err)
	}

	joinURL := &url.URL{
		Scheme: "https",
		Host:   net.JoinHostPort(c.Host, c.Port),
		Path:   fmt.Sprintf(JoinNetworkPath, networkID),
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, joinURL.String(), nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	err = internal.BackoffRetry(ctx, func() error {
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
func (c *Client) GetNetworkConfiguration(ctx context.Context, vnetID string) (VirtualNetwork, error) {
	requestStart := time.Now()

	path := &url.URL{
		Scheme: "https",
		Host:   net.JoinHostPort(c.Host, c.Port),
		Path:   fmt.Sprintf(GetNetworkConfigPath, vnetID),
	}

	var out VirtualNetwork

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path.String(), nil)
	if err != nil {
		return out, fmt.Errorf("creating http request to %q: %w", path.String(), err)
	}

	err = internal.BackoffRetry(ctx, func() error {
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

	req, err := http.NewRequest(http.MethodPost, path.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("submitting request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return c.error(time.Since(requestStart), resp.StatusCode)
	}
	return nil
}

func (c *Client) DeleteNetworkContainer(ctx context.Context) error {
	return nil
}

func (c *Client) GetNmAgentSupportedApiURLFmt(ctx context.Context) error {
	return nil
}

func (c *Client) hostPort() string {
	return net.JoinHostPort(c.Host, c.Port)
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
