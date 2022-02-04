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

// RecordPolicyExecTime adds an observation of policy exec time for the specified operation.
// The execution time is from the timer's start until now.
func RecordPolicyExecTime(timer *Timer, op OperationKind) {
	if op == CreateOp {
		timer.stopAndRecord(addPolicyExecTime)
	} else {
		timer.stopAndRecordCRUDExecTime(controllerPolicyExecTime, op)
	}
}

// GetNumPolicies returns the number of policies.
// This function is slow.
func GetNumPolicies() (int, error) {
	return getValue(numPolicies)
}

// GetPolicyExecCount returns the number of observations for policy exec time for the specified operation.
// This function is slow.
func GetPolicyExecCount(op OperationKind) (int, error) {
	if op == CreateOp {
		return getCountValue(addPolicyExecTime)
	}
	labels := prometheus.Labels{operationLabel: string(op)}
	return getCountVecValue(controllerPolicyExecTime, labels)
}
