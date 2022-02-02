package policies

import (
	"fmt"
	"sync"
	"time"

	"github.com/Azure/azure-container-networking/common"
	"github.com/Azure/azure-container-networking/npm/metrics"
	npmerrors "github.com/Azure/azure-container-networking/npm/util/errors"
	"k8s.io/klog"
)

// PolicyManagerMode will be used in windows to decide if
// SetPolicies should be used or not
type PolicyManagerMode string

const (
	// IPSetPolicyMode will references IPSets in policies
	IPSetPolicyMode PolicyManagerMode = "IPSet"
	// IPPolicyMode will replace ipset names with their value IPs in policies
	IPPolicyMode PolicyManagerMode = "IP"

	reconcileTimeInMinutes = 5
)

type PolicyManagerCfg struct {
	// PolicyMode only affects Windows
	PolicyMode PolicyManagerMode
}

type PolicyMap struct {
	cache map[string]*NPMNetworkPolicy
}

type reconcileManager struct {
	sync.Mutex
	releaseLockSignal chan struct{}
}

type PolicyManager struct {
	policyMap        *PolicyMap
	ioShim           *common.IOShim
	staleChains      *staleChains
	reconcileManager *reconcileManager
	*PolicyManagerCfg
}

func NewPolicyManager(ioShim *common.IOShim, cfg *PolicyManagerCfg) *PolicyManager {
	return &PolicyManager{
		policyMap: &PolicyMap{
			cache: make(map[string]*NPMNetworkPolicy),
		},
		ioShim:      ioShim,
		staleChains: newStaleChains(),
		reconcileManager: &reconcileManager{
			releaseLockSignal: make(chan struct{}, 1),
		},
		PolicyManagerCfg: cfg,
	}
}

func (pMgr *PolicyManager) Bootup(epIDs []string) error {
	if err := pMgr.bootup(epIDs); err != nil {
		return npmerrors.ErrorWrapper(npmerrors.BootupPolicyMgr, false, "failed to bootup policy manager", err)
	}
	return nil
}

func (pMgr *PolicyManager) Reconcile(stopChannel <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(time.Minute * time.Duration(reconcileTimeInMinutes))
		defer ticker.Stop()

		for {
			select {
			case <-stopChannel:
				return
			case <-ticker.C:
				pMgr.reconcile()
			}
		}
	}()
}

func (pMgr *PolicyManager) PolicyExists(policyKey string) bool {
	_, ok := pMgr.policyMap.cache[policyKey]
	return ok
}

func (pMgr *PolicyManager) GetPolicy(policyKey string) (*NPMNetworkPolicy, bool) {
	policy, ok := pMgr.policyMap.cache[policyKey]
	return policy, ok
}

func (pMgr *PolicyManager) AddPolicy(policy *NPMNetworkPolicy, endpointList map[string]string) error {
	prometheusTimer := metrics.StartNewTimer()
	if len(policy.ACLs) == 0 {
		klog.Infof("[DataPlane] No ACLs in policy %s to apply", policy.PolicyKey)
		return nil
	}
	defer metrics.RecordACLRuleExecTime(prometheusTimer) // record execution time regardless of failure
	NormalizePolicy(policy)
	if err := ValidatePolicy(policy); err != nil {
		return npmerrors.Errorf(npmerrors.AddPolicy, false, fmt.Sprintf("couldn't add malformed policy: %s", err.Error()))
	}

	// Call actual dataplane function to apply changes
	err := pMgr.addPolicy(policy, endpointList)
	if err != nil {
		return npmerrors.Errorf(npmerrors.AddPolicy, false, fmt.Sprintf("failed to add policy: %v", err))
	}

	pMgr.policyMap.cache[policy.PolicyKey] = policy
	return nil
}

func (pMgr *PolicyManager) isFirstPolicy() bool {
	return len(pMgr.policyMap.cache) == 0
}

func (pMgr *PolicyManager) RemovePolicy(policyKey string, endpointList map[string]string) error {
	policy, ok := pMgr.GetPolicy(policyKey)

	if !ok {
		return nil
	}

	if len(policy.ACLs) == 0 {
		klog.Infof("[DataPlane] No ACLs in policy %s to remove", policyKey)
		return nil
	}
	// Call actual dataplane function to apply changes
	err := pMgr.removePolicy(policy, endpointList)
	if err != nil {
		return npmerrors.Errorf(npmerrors.RemovePolicy, false, fmt.Sprintf("failed to remove policy: %v", err))
	}

	delete(pMgr.policyMap.cache, policyKey)
	return nil
}

func (pMgr *PolicyManager) isLastPolicy() bool {
	// if we change our code to delete more than one policy at once, we can specify numPoliciesToDelete as an argument
	numPoliciesToDelete := 1
	return len(pMgr.policyMap.cache) == numPoliciesToDelete
}
