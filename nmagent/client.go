package nmagent

import (
	"context"
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

	// TODO(timraymond): exponential backoff needed
	for {
		// check to see if the context is still alive
		if err := ctx.Err(); err != nil {
			return err
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return fmt.Errorf("executing request: %w", err)
		}
		defer resp.Body.Close()

		// the response from NMAgent only contains the HTTP status code, so there is
		// no need to parse it

		switch resp.StatusCode {
		case http.StatusOK:
			return nil
		case http.StatusInternalServerError:
			return Error{
				Code: http.StatusInternalServerError,
			}
		case http.StatusProcessing:
			continue
		default:
			return nil
		}
	}
}

// GetNetworkConfiguration retrieves the configuration of a customer's virtual
// network. Only subnets which have been delegated will be returned
func (c *Client) GetNetworkConfiguration(ctx context.Context, vnetID string) (VirtualNetwork, error) {
	path := &url.URL{
		Scheme: "https",
		Host:   net.JoinHostPort(c.Host, c.Port),
		Path:   fmt.Sprintf(GetNetworkConfigPath, vnetID),
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path.String(), strings.NewReader(""))
	if err != nil {
		return VirtualNetwork{}, fmt.Errorf("creating http request to %q: %w", path.String(), err)
	}

	for {
		// check to see if the context is dead
		if err := ctx.Err(); err != nil {
			return VirtualNetwork{}, err
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return VirtualNetwork{}, fmt.Errorf("executing http request to %q: %w", path.String(), err)
		}
		defer resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusOK:
			var out VirtualNetwork
			err = json.NewDecoder(resp.Body).Decode(&out)
			if err != nil {
				return VirtualNetwork{}, fmt.Errorf("decoding json response for %q: %w", path.String(), err)
			}
			return out, nil
		case http.StatusProcessing:
			continue
		default:
			return VirtualNetwork{}, fmt.Errorf("unexpected HTTP status from NMAgent (%d): %s", resp.StatusCode, http.StatusText(resp.StatusCode))
		}
	}
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
