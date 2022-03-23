package nmagent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

// PutNetworkContainerRequest {{{1

// PutNetworkContainerRequest is a collection of parameters necessary to create
// a new network container
type PutNetworkContainerRequest struct {
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

	// VirtualNetworkID is the ID of the customer's virtual network
	VirtualNetworkID string `json:"virtualNetworkId"`

	// AuthenticationToken is the base64 security token for the subnet containing
	// the Network Container addresses
	AuthenticationToken string `json:"-"`

	// PrimaryAddress is the primary customer address of the interface in the
	// management VNet
	PrimaryAddress string `json:"-"`
}

// Path returns the URL path necessary to submit this PutNetworkContainerRequest
func (p PutNetworkContainerRequest) Path() string {
	const PutNCRequestPath string = "/NetworkManagement/interfaces/%s/networkContainers/%s/authenticationToken/%s/api-version/1"
	return fmt.Sprintf(PutNCRequestPath, p.PrimaryAddress, p.ID, p.AuthenticationToken)
}

// Validate ensures that all of the required parameters of the request have
// been filled out properly prior to submission to NMAgent
func (p PutNetworkContainerRequest) Validate() error {
	var errs ValidationError

	if len(p.IPv4Addrs) == 0 {
		errs.MissingFields = append(errs.MissingFields, "IPv4Addrs")
	}

	if p.SubnetName == "" {
		errs.MissingFields = append(errs.MissingFields, "SubnetName")
	}

	// it's a little unclear as to whether a version value of "0" is actually
	// legal. Given that this is the zero value of this field, and the
	// documentation of NMAgent requires this to be a uint64, we'll consider "0"
	// as unset and require it to be something else.
	if p.Version == uint64(0) {
		errs.MissingFields = append(errs.MissingFields, "Version")
	}

	if p.VirtualNetworkID == "" {
		errs.MissingFields = append(errs.MissingFields, "VirtualNetworkID")
	}

	if errs.IsEmpty() {
		return nil
	}
	return errs
}

// Policy {{{2

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

// }}}2

// }}}1

// JoinNetworkRequest {{{1

type JoinNetworkRequest struct {
	NetworkID string `json:"-"` // the customer's VNet ID
}

// Path constructs a URL path for invoking a JoinNetworkRequest using the
// provided parameters
func (j JoinNetworkRequest) Path() string {
	const JoinNetworkPath string = "/NetworkManagement/joinedVirtualNetworks/%s/api-version/1"
	return fmt.Sprintf(JoinNetworkPath, j.NetworkID)
}

// Validate ensures that the provided parameters of the request are valid
func (j JoinNetworkRequest) Validate() error {
	// we need to be a little defensive, because there is no bad request response
	// from NMAgent
	if _, err := uuid.Parse(j.NetworkID); err != nil {
		return fmt.Errorf("bad network ID %q: %w", j.NetworkID, err)
	}
	return nil
}

// }}}1

// DeleteNetworkRequest {{{1

// DeleteContainerRequest represents all information necessary to request that
// NMAgent delete a particular network container
type DeleteContainerRequest struct {
	NCID string `json:"-"` // the Network Container ID

	// PrimaryAddress is the primary customer address of the interface in the
	// management VNET
	PrimaryAddress      string `json:"-"`
	AuthenticationToken string `json:"-"`
}

// Path returns the path for submitting a DeleteContainerRequest with
// parameters interpolated correctly
func (d DeleteContainerRequest) Path() string {
	const DeleteNCPath string = "/NetworkManagement/interfaces/%s/networkContainers/%s/authenticationToken/%s/api-version/1/method/DELETE"
	return fmt.Sprintf(DeleteNCPath, d.PrimaryAddress, d.NCID, d.AuthenticationToken)
}

// Validate ensures that the DeleteContainerRequest has the correct information
// to submit the request
func (d DeleteContainerRequest) Validate() error {
	errs := ValidationError{}

	if d.NCID == "" {
		errs.MissingFields = append(errs.MissingFields, "NCID")
	}

	if d.PrimaryAddress == "" {
		errs.MissingFields = append(errs.MissingFields, "PrimaryAddress")
	}

	if d.AuthenticationToken == "" {
		errs.MissingFields = append(errs.MissingFields, "AuthenticationToken")
	}

	if !errs.IsEmpty() {
		return errs
	}

	return nil
}

// }}}1

// GetNetworkConfigRequest {{{1

// GetNetworkConfigRequest is a collection of necessary information for
// submitting a request for a customer's network configuration
type GetNetworkConfigRequest struct {
	VNetID string `json:"-"` // the customer's virtual network ID
}

// Path produces a URL path used to submit a request
func (g GetNetworkConfigRequest) Path() string {
	const GetNetworkConfigPath string = "/NetworkManagement/joinedVirtualNetworks/%s/api-version/1"
	return fmt.Sprintf(GetNetworkConfigPath, g.VNetID)
}

// Validate ensures that the request is complete and the parameters are correct
func (g GetNetworkConfigRequest) Validate() error {
	errs := ValidationError{}

	if g.VNetID == "" {
		errs.MissingFields = append(errs.MissingFields, "VNetID")
	}

	if !errs.IsEmpty() {
		return errs
	}

	return nil
}

// }}}1

// ValidationError {{{1

type ValidationError struct {
	MissingFields []string
}

func (v ValidationError) Error() string {
	return fmt.Sprintf("missing fields: %s", strings.Join(v.MissingFields, ", "))
}

func (v ValidationError) IsEmpty() bool {
	return len(v.MissingFields) == 0
}

// }}}1
