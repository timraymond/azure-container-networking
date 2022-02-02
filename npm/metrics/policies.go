package metrics

import "github.com/prometheus/client_golang/prometheus"

// IncNumPolicies increments the number of policies.
func IncNumPolicies() {
	numPolicies.Inc()
}

// DecNumPolicies decrements the number of policies.
func DecNumPolicies() {
	numPolicies.Dec()
}

// ResetNumPolicies sets the number of policies to 0.
func ResetNumPolicies() {
	numPolicies.Set(0)
}

// RecordPolicyApplyTime adds an observation of policy apply time for the specified apply mode.
// The execution time is from the timer's start until now.
func RecordPolicyApplyTime(timer *Timer, mode ApplyMode) {
	if mode == CreateMode {
		timer.stopAndRecord(addPolicyExecTime)
	} else {
		timer.stopAndRecordApplyTime(policyApplyTime, mode)
	}
}

// GetNumPolicies returns the number of policies.
// This function is slow.
func GetNumPolicies() (int, error) {
	return getValue(numPolicies)
}

// GetPolicyApplyCount returns the number of observations for policy apply time for the specified apply mode.
// This function is slow.
func GetPolicyApplyCount(mode ApplyMode) (int, error) {
	if mode == CreateMode {
		return getCountValue(addPolicyExecTime)
	}
	labels := prometheus.Labels{applyModeLabel: string(mode)}
	return getCountValue(policyApplyTime.With(labels))
}
