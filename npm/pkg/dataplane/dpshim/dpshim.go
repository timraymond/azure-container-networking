package dpshim

import (
	"errors"
	"fmt"
	"sync"

	"github.com/Azure/azure-container-networking/npm/pkg/controlplane"
	"github.com/Azure/azure-container-networking/npm/pkg/dataplane"
	"github.com/Azure/azure-container-networking/npm/pkg/dataplane/ipsets"
	"github.com/Azure/azure-container-networking/npm/pkg/dataplane/policies"
	"github.com/Azure/azure-container-networking/npm/pkg/protos"
)

var ErrChannelUnset = errors.New("channel must be set")

// TODO setting this up to unblock another workitem
type DPShim struct {
	outChannel  chan *protos.Events
	setCache    map[string]*controlplane.ControllerIPSets
	policyCache map[string]*policies.NPMNetworkPolicy
	sync.Mutex
}

func NewDPSim(outChannel chan *protos.Events) (*DPShim, error) {
	if outChannel == nil {
		return nil, fmt.Errorf("out channel must be set: %w", ErrChannelUnset)
	}

	return &DPShim{
		outChannel:  outChannel,
		setCache:    make(map[string]*controlplane.ControllerIPSets),
		policyCache: make(map[string]*policies.NPMNetworkPolicy),
	}, nil
}

func (dp *DPShim) ResetDataPlane() error {
	return nil
}

func (dp *DPShim) RunPeriodicTasks() {}

func (dp *DPShim) GetIPSet(setName string) *ipsets.IPSet {
	return nil
}

func (dp *DPShim) setExists(setName string) bool {
	_, ok := dp.setCache[setName]
	return ok
}

func (dp *DPShim) CreateIPSets(setNames []*ipsets.IPSetMetadata) {
	dp.Lock()
	defer dp.Unlock()
	for _, set := range setNames {
		dp.createIPSets(set)
	}
}

func (dp *DPShim) createIPSets(set *ipsets.IPSetMetadata) {
	setName := set.GetPrefixName()

	if dp.setExists(setName) {
		return
	}

	dp.setCache[setName] = controlplane.NewControllerIPSets(set)
}

func (dp *DPShim) DeleteIPSet(setMetadata *ipsets.IPSetMetadata) {
	dp.Lock()
	defer dp.Unlock()
	dp.deleteIPSet(setMetadata)
}

func (dp *DPShim) deleteIPSet(setMetadata *ipsets.IPSetMetadata) {
	set, ok := dp.setCache[setMetadata.GetPrefixName()]
	if !ok {
		return
	}

	if set.HasReferences() {
		return
	}

	delete(dp.setCache, setMetadata.GetPrefixName())
}

func (dp *DPShim) AddToSets(setNames []*ipsets.IPSetMetadata, podMetadata *dataplane.PodMetadata) error {
	dp.Lock()
	defer dp.Unlock()
	return nil
}

func (dp *DPShim) RemoveFromSets(setNames []*ipsets.IPSetMetadata, podMetadata *dataplane.PodMetadata) error {
	dp.Lock()
	defer dp.Unlock()
	return nil
}

func (dp *DPShim) AddToLists(listName, setNames []*ipsets.IPSetMetadata) error {
	dp.Lock()
	defer dp.Unlock()
	return nil
}

func (dp *DPShim) RemoveFromList(listName *ipsets.IPSetMetadata, setNames []*ipsets.IPSetMetadata) error {
	dp.Lock()
	defer dp.Unlock()
	return nil
}

func (dp *DPShim) ApplyDataPlane() error {
	return nil
}

func (dp *DPShim) AddPolicy(networkpolicies *policies.NPMNetworkPolicy) error {
	var err error
	// apply dataplane after syncing
	defer func() {
		dperr := dp.ApplyDataPlane()
		if dperr != nil {
			err = fmt.Errorf("failed with error %w, apply failed with %v", err, dperr)
		}
	}()
	dp.Lock()
	defer dp.Unlock()

	if dp.policyExists(networkpolicies.PolicyKey) {
		return nil
	}
	dp.policyCache[networkpolicies.PolicyKey] = networkpolicies
	return err
}

func (dp *DPShim) RemovePolicy(policyName string) error {
	var err error
	// apply dataplane after syncing
	defer func() {
		dperr := dp.ApplyDataPlane()
		if dperr != nil {
			err = fmt.Errorf("failed with error %w, apply failed with %v", err, dperr)
		}
	}()

	dp.Lock()
	defer dp.Unlock()
	// keeping err different so we can catch the defer func err
	delete(dp.policyCache, policyName)
	return err
}

func (dp *DPShim) UpdatePolicy(networkpolicies *policies.NPMNetworkPolicy) error {
	var err error
	// apply dataplane after syncing
	defer func() {
		dperr := dp.ApplyDataPlane()
		if dperr != nil {
			err = fmt.Errorf("failed with error %w, apply failed with %v", err, dperr)
		}
	}()

	dp.Lock()
	defer dp.Unlock()

	dp.policyCache[networkpolicies.PolicyKey] = networkpolicies

	return err
}

func (dp *DPShim) policyExists(policyName string) bool {
	_, ok := dp.policyCache[policyName]
	return ok
}
