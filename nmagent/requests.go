package nmagent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

type Policy struct {
	ID   string
	Type string
}

// MarshalJson encodes policies as a JSON string, separated by a comma. This
// specific format is requested by the NMAgent documentation
func (p Policy) MarshalJSON() ([]byte, error) {
	out := bytes.NewBufferString(p.ID)
	out.WriteString(", ")
	out.WriteString(p.Type)

	outStr := out.String()
	return json.Marshal(outStr)
}

// UnmarshalJSON decodes a JSON-encoded policy string
func (p *Policy) UnmarshalJSON(in []byte) error {
	var raw string
	err := json.Unmarshal(in, &raw)
	if err != nil {
		return fmt.Errorf("decoding policy: %w", err)
	}

	parts := strings.Split(raw, ",")
	if len(parts) != 2 {
		return fmt.Errorf("policies must be two comma-separated values")
	}

	p.ID = strings.TrimFunc(parts[0], unicode.IsSpace)
	p.Type = strings.TrimFunc(parts[1], unicode.IsSpace)

	return nil
}

type NetworkContainerRequest struct {
	ID      string `json:"networkContainerID"` // the id of the network container
	VNetID  string `json:"virtualNetworkID"`   // the id of the customer's vnet
	Version uint64 `json:"version"`            // the new network container version

	// SubnetName is the name of the delegated subnet. This is used to
	// authenticate the request. The list of ipv4addresses must be contained in
	// the subnet's prefix.
	SubnetName string `json:"subnetName"`

	// IPv4 addresses in the customer virtual network that will be assigned to
	// the interface.
	IPv4Addrs []string `json:"ipV4Addresses"`

	Policies []Policy `json:"policies"` // policies applied to the network container

	// VlanID is used to distinguish Network Containers with duplicate customer
	// addresses. "0" is considered a default value by the API.
	VlanID int `json:"vlanId"`

	// AuthenticationToken is the base64 security token for the subnet containing
	// the Network Container addresses
	AuthenticationToken string `json:"-"`

	// PrimaryAddress is the primary customer address of the interface in the
	// management VNet
	PrimaryAddress string `json:"-"`
}
