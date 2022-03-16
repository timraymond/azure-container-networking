package nmagent

import (
	"context"
	"dnc/nmagent/internal"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

const (
	JoinNetworkPath      string = "/machine/plugins/?comp=nmagent&type=NetworkManagement/joinedVirtualNetworks/%s/api-version/1"
	GetNetworkConfigPath string = "/machine/plugins/?comp=nmagent&type=NetworkManagement/joinedVirtualNetworks/%s/api-version/1"
)

// Error is a aberrent condition encountered when interacting with the NMAgent
// API
type Error struct {
	Code int // the HTTP status code received
}

func (e Error) Error() string {
	return fmt.Sprintf("nmagent: http status %d", e.Code)
}

// Temporary reports whether the error encountered from NMAgent should be
// considered temporary, and thus retriable
func (e Error) Temporary() bool {
	return e.Code == http.StatusProcessing
}

type VirtualNetwork struct {
	CNetSpace      string   `json:"cnetSpace"`
	DefaultGateway string   `json:"defaultGateway"`
	DNSServers     []string `json:"dnsServers"`
	Subnets        []Subnet `json:"subnets"`
	VNetSpace      string   `json:"vnetSpace"`
	VNetVersion    string   `json:"vnetVersion"`
}

type Subnet struct {
	AddressPrefix string `json:"addressPrefix"`
	SubnetName    string `json:"subnetName"`
	Tags          []Tag  `json:"tags"`
}

type Tag struct {
	Name string `json:"name"`
	Type string `json:"type"` // the type of the tag (e.g. "System" or "Custom")
}

// Client is an agent for exchanging information with NMAgent
type Client struct {
	HTTPClient *http.Client

	// config
	Host string
	Port string
}

// JoinNetwork joins a node to a customer's virtual network
func (c *Client) JoinNetwork(ctx context.Context, networkID string) error {
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, joinURL.String(), strings.NewReader(""))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	err = internal.BackoffRetry(ctx, func() error {
		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return fmt.Errorf("executing request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return Error{resp.StatusCode}
		}
		return nil
	})

	return err
}

// GetNetworkConfiguration retrieves the configuration of a customer's virtual
// network. Only subnets which have been delegated will be returned
func (c *Client) GetNetworkConfiguration(ctx context.Context, vnetID string) (VirtualNetwork, error) {
	path := &url.URL{
		Scheme: "https",
		Host:   net.JoinHostPort(c.Host, c.Port),
		Path:   fmt.Sprintf(GetNetworkConfigPath, vnetID),
	}

	var out VirtualNetwork

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path.String(), strings.NewReader(""))
	if err != nil {
		return out, fmt.Errorf("creating http request to %q: %w", path.String(), err)
	}

	err = internal.BackoffRetry(ctx, func() error {
		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return fmt.Errorf("executing http request to %q: %w", path.String(), err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return Error{resp.StatusCode}
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

/*
func (c *Client) PutNetworkContainer(ctx context.Context) error {
	return nil
}

func (c *Client) DeleteNetworkContainer(ctx context.Context) error {
	return nil
}

func (c *Client) GetNmAgentSupportedApiURLFmt(ctx context.Context) error {
	return nil
}
*/
