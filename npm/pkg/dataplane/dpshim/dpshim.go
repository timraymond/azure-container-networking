package dpshim

import (
	"bytes"
	"errors"
	"fmt"
	"sync"

	"github.com/Azure/azure-container-networking/npm/pkg/controlplane"
	"github.com/Azure/azure-container-networking/npm/pkg/dataplane"
	"github.com/Azure/azure-container-networking/npm/pkg/dataplane/ipsets"
	"github.com/Azure/azure-container-networking/npm/pkg/dataplane/policies"
	"github.com/Azure/azure-container-networking/npm/pkg/protos"
	npmerrors "github.com/Azure/azure-container-networking/npm/util/errors"
	"github.com/Azure/azure-container-networking/vendor/k8s.io/klog"
)

var ErrChannelUnset = errors.New("channel must be set")

type DPShim struct {
	outChannel  chan *protos.Events
	setCache    map[string]*controlplane.ControllerIPSets
	policyCache map[string]*policies.NPMNetworkPolicy
	dirtyCache  *dirtyCache
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
		dirtyCache:  newDirtyCache(),
	}, nil
}

func (dp *DPShim) ResetDataPlane() error {
	return nil
}

func (dp *DPShim) RunPeriodicTasks() {}

func (dp *DPShim) GetIPSet(setName string) *ipsets.IPSet {
	return nil
}

func (dp *DPShim) getIPSet(setName string) *controlplane.ControllerIPSets {
	return dp.setCache[setName]
}

func (dp *DPShim) setExists(setName string) bool {
	_, ok := dp.setCache[setName]
	return ok
}

func (dp *DPShim) CreateIPSets(setNames []*ipsets.IPSetMetadata) {
	dp.Lock()
	defer dp.Unlock()
	for _, set := range setNames {
		dp.createIPSet(set)
	}
}

func (dp *DPShim) createIPSet(set *ipsets.IPSetMetadata) {
	setName := set.GetPrefixName()

	if dp.setExists(setName) {
		return
	}

	dp.setCache[setName] = controlplane.NewControllerIPSets(set)
	dp.dirtyCache.modifyAddorUpdateSets(setName)
}

func (dp *DPShim) DeleteIPSet(setMetadata *ipsets.IPSetMetadata) {
	dp.Lock()
	defer dp.Unlock()
	dp.deleteIPSet(setMetadata)
}

func (dp *DPShim) deleteIPSet(setMetadata *ipsets.IPSetMetadata) {
	setName := setMetadata.GetPrefixName()
	set, ok := dp.setCache[setName]
	if !ok {
		return
	}

	if set.HasReferences() {
		return
	}

	delete(dp.setCache, setName)
	dp.dirtyCache.modifyDeleteSets(setName)
}

func (dp *DPShim) AddToSets(setMetadatas []*ipsets.IPSetMetadata, podMetadata *dataplane.PodMetadata) error {
	dp.Lock()
	defer dp.Unlock()
	for _, set := range setMetadatas {
		prefixedSetName := set.GetPrefixName()
		if !dp.setExists(prefixedSetName) {
			dp.createIPSet(set)
		}

		set := dp.setCache[prefixedSetName]
		if set.IPSetMetadata.GetSetKind() != ipsets.HashSet {
			return npmerrors.Errorf(npmerrors.AppendIPSet, false, fmt.Sprintf("ipset %s is not a hash set", prefixedSetName))
		}

		cachedPod, ok := set.IPPodMetadata[podMetadata.PodIP]
		set.IPPodMetadata[podMetadata.PodIP] = podMetadata
		if ok && cachedPod.PodKey != podMetadata.PodKey {
			klog.Infof("AddToSet: PodOwner has changed for Ip: %s, setName:%s, Old podKey: %s, new podKey: %s. Replace context with new PodOwner.",
				cachedPod.PodIP, set.Name, cachedPod.PodKey, podMetadata.PodKey)
			continue
		}

		dp.dirtyCache.modifyAddorUpdateSets(prefixedSetName)
	}

	return nil
}

func (dp *DPShim) RemoveFromSets(setMetadatas []*ipsets.IPSetMetadata, podMetadata *dataplane.PodMetadata) error {
	dp.Lock()
	defer dp.Unlock()
	for _, set := range setMetadatas {
		prefixedSetName := set.GetPrefixName()
		if !dp.setExists(prefixedSetName) {
			dp.createIPSet(set)
		}

		set := dp.setCache[prefixedSetName]
		if set.IPSetMetadata.GetSetKind() != ipsets.HashSet {
			return npmerrors.Errorf(npmerrors.AppendIPSet, false, fmt.Sprintf("RemoveFromSets, ipset %s is not a hash set", prefixedSetName))
		}

		// in case the IP belongs to a new Pod, then ignore this Delete call as this might be stale
		cachedPod, exists := set.IPPodMetadata[podMetadata.PodIP]
		if !exists {
			continue
		}
		if cachedPod.PodKey != podMetadata.PodKey {
			klog.Infof("DeleteFromSet: PodOwner has changed for Ip: %s, setName:%s, Old podKey: %s, new podKey: %s. Ignore the delete as this is stale update",
				cachedPod.PodIP, prefixedSetName, cachedPod.PodKey, podMetadata.PodKey)
			continue
		}

		// update the IP ownership with podkey
		delete(set.IPPodMetadata, podMetadata.PodIP)
		dp.dirtyCache.modifyAddorUpdateSets(prefixedSetName)
	}
	return nil
}

func (dp *DPShim) AddToLists(listMetadatas, setMetadatas []*ipsets.IPSetMetadata) error {
	dp.Lock()
	defer dp.Unlock()

	if err := dp.checkForListMemberUpdateErrors(listMetadatas, setMetadatas, npmerrors.AppendIPSet); err != nil {
		return err
	}

	for _, listMetadata := range listMetadatas {
		listName := listMetadata.GetPrefixName()
		list := dp.setCache[listName]
		for _, setMetadata := range setMetadatas {
			setName := setMetadata.GetPrefixName()
			set := dp.setCache[setName]

			if _, ok := list.MemberIPSets[setName]; ok {
				continue
			}
			list.MemberIPSets[setName] = set.IPSetMetadata
			set.AddReference(listName, controlplane.ListReference)
			dp.dirtyCache.modifyAddorUpdateSets(setName)
		}
		dp.dirtyCache.modifyAddorUpdateSets(listName)
	}

	return nil
}

func (dp *DPShim) RemoveFromList(listMetadata *ipsets.IPSetMetadata, setMetadatas []*ipsets.IPSetMetadata) error {
	dp.Lock()
	defer dp.Unlock()

	if err := dp.checkForListMemberUpdateErrors([]*ipsets.IPSetMetadata{listMetadata}, setMetadatas, npmerrors.DeleteIPSet); err != nil {
		return err
	}

	listName := listMetadata.GetPrefixName()
	list := dp.setCache[listName]
	for _, setMetadata := range setMetadatas {
		setName := setMetadata.GetPrefixName()
		set := dp.setCache[setName]

		if _, ok := list.MemberIPSets[setName]; !ok {
			continue
		}

		delete(list.MemberIPSets, setName)
		set.DeleteReference(listName, controlplane.ListReference)
	}
	dp.dirtyCache.modifyAddorUpdateSets(listName)

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
	dp.dirtyCache.modifyAddorUpdatePolicies(networkpolicies.PolicyKey)

	return err
}

func (dp *DPShim) RemovePolicy(PolicyKey string) error {
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
	delete(dp.policyCache, PolicyKey)
	dp.dirtyCache.modifyDeletePolicies(PolicyKey)

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

	// For simplicity, we will not be adding references of netpols to ipsets.
	// DP in daemon will take care of tracking the references.

	dp.policyCache[networkpolicies.PolicyKey] = networkpolicies
	dp.dirtyCache.modifyAddorUpdatePolicies(networkpolicies.PolicyKey)

	return err
}

func (dp *DPShim) ApplyDataPlane() error {
	dp.Lock()
	defer dp.Unlock()

	// check dirty cache contents
	if !dp.dirtyCache.hasContents() {
		klog.Info("ApplyDataPlane: No changes to apply")
		return nil
	}

	goalStates := make(map[string]*protos.GoalState)

	toApplySets, err := dp.processIPSetsApply()
	if err != nil {
		return err
	}
	if toApplySets != nil {
		goalStates[controlplane.IpsetApply] = toApplySets
	}

	toDeleteSets, err := dp.processIPSetsDelete()
	if err != nil {
		return err
	}
	if toDeleteSets != nil {
		goalStates[controlplane.IpsetRemove] = toDeleteSets
	}

	toApplyPolicies, err := dp.processPoliciesApply()
	if err != nil {
		return err
	}
	if toApplyPolicies != nil {
		goalStates[controlplane.PolicyApply] = toApplyPolicies
	}

	toDeletePolicies, err := dp.processPoliciesRemove()
	if err != nil {
		return err
	}
	if toDeletePolicies != nil {
		goalStates[controlplane.PolicyRemove] = toDeletePolicies
	}

	if len(goalStates) == 0 {
		klog.Info("ApplyDataPlane: No changes to apply")
		return nil
	}

	go func() {
		dp.outChannel <- &protos.Events{
			Payload: goalStates,
		}
	}()

	return nil
}

func (dp *DPShim) policyExists(PolicyKey string) bool {
	_, ok := dp.policyCache[PolicyKey]
	return ok
}

func (dp *DPShim) checkForListMemberUpdateErrors(listMetadata, memberMetadatas []*ipsets.IPSetMetadata, npmErrorString string) error {
	for _, listMetadata := range listMetadata {
		prefixedListName := listMetadata.GetPrefixName()
		if !dp.setExists(prefixedListName) {
			dp.createIPSet(listMetadata)
		}

		list := dp.setCache[prefixedListName]
		if list.IPSetMetadata.GetSetKind() != ipsets.ListSet {
			return npmerrors.Errorf(npmErrorString, false, fmt.Sprintf("ipset %s is not a list set", prefixedListName))
		}
	}

	for _, memberMetadata := range memberMetadatas {
		memberName := memberMetadata.GetPrefixName()
		if !dp.setExists(memberName) {
			dp.createIPSet(memberMetadata)
		}
		member := dp.setCache[memberName]

		// Nested IPSets are only supported for windows
		// Check if we want to actually use that support
		if member.IPSetMetadata.GetSetKind() != ipsets.HashSet {
			return npmerrors.Errorf(npmErrorString, false, fmt.Sprintf("ipset %s is not a hash set and nested list sets are not supported", memberName))
		}
	}
	return nil
}

func (dp *DPShim) processIPSetsApply() (*protos.GoalState, error) {
	if len(dp.dirtyCache.toAddorUpdateSets) == 0 {
		return nil, nil
	}

	toApplySets := make([]*controlplane.ControllerIPSets, len(dp.dirtyCache.toAddorUpdateSets))
	idx := 0

	for setName := range dp.dirtyCache.toAddorUpdateSets {
		set := dp.getIPSet(setName)
		if set == nil {
			klog.Errorf("processIPSetsApply: set %s not found", setName)
			return nil, npmerrors.Errorf(npmerrors.AppendIPSet, false, fmt.Sprintf("ipset %s not found", setName))
		}

		toApplySets[idx] = set
		idx++
	}

	payload, err := controlplane.EncodeControllerIPSets(toApplySets)
	if err != nil {
		klog.Errorf("processIPSetsApply: failed to encode sets %v", err)
		return nil, err
	}

	return getGoalStateFromBuffer(payload), nil
}

func (dp *DPShim) processIPSetsDelete() (*protos.GoalState, error) {
	if len(dp.dirtyCache.toDeleteSets) == 0 {
		return nil, nil
	}

	toDeleteSets := make([]string, len(dp.dirtyCache.toDeleteSets))
	idx := 0

	for setName := range dp.dirtyCache.toDeleteSets {
		toDeleteSets[idx] = setName
		idx++
	}

	payload, err := controlplane.EncodeStrings(toDeleteSets)
	if err != nil {
		klog.Errorf("processIPSetsDelete: failed to encode sets %v", err)
		return nil, err
	}

	return getGoalStateFromBuffer(payload), nil
}

func (dp *DPShim) processPoliciesApply() (*protos.GoalState, error) {
	if len(dp.dirtyCache.toAddorUpdatePolicies) == 0 {
		return nil, nil
	}

	toApplyPolicies := make([]*policies.NPMNetworkPolicy, len(dp.dirtyCache.toAddorUpdatePolicies))
	idx := 0

	for policyKey := range dp.dirtyCache.toAddorUpdatePolicies {
		if !dp.policyExists(policyKey) {
			return nil, npmerrors.Errorf(npmerrors.AddPolicy, false, fmt.Sprintf("policy %s not found", policyKey))
		}

		policy := dp.policyCache[policyKey]
		toApplyPolicies[idx] = policy
		idx++
	}

	payload, err := controlplane.EncodeNPMNetworkPolicies(toApplyPolicies)
	if err != nil {
		klog.Errorf("processPoliciesApply: failed to encode policies %v", err)
		return nil, err
	}

	return getGoalStateFromBuffer(payload), nil
}

func (dp *DPShim) processPoliciesRemove() (*protos.GoalState, error) {
	if len(dp.dirtyCache.toDeletePolicies) == 0 {
		return nil, nil
	}

	toDeletePolicies := make([]string, len(dp.dirtyCache.toDeletePolicies))
	idx := 0

	for policyKey := range dp.dirtyCache.toDeletePolicies {
		toDeletePolicies[idx] = policyKey
		idx++
	}

	payload, err := controlplane.EncodeStrings(toDeletePolicies)
	if err != nil {
		klog.Errorf("processPoliciesRemove: failed to encode policies %v", err)
		return nil, err
	}

	return getGoalStateFromBuffer(payload), nil
}

func getGoalStateFromBuffer(payload *bytes.Buffer) *protos.GoalState {
	return &protos.GoalState{
		Data: payload.Bytes(),
	}
}
