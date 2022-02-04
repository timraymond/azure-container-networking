package metrics

import "testing"

var numPoliciesMetric = &basicMetric{ResetNumPolicies, IncNumPolicies, DecNumPolicies, GetNumPolicies}

func TestRecordPolicyExecTime(t *testing.T) {
	testStopAndRecordCRUDExecTime(t, &crudExecMetric{
		RecordPolicyExecTime,
		GetPolicyExecCount,
	})
}

func TestIncNumPolicies(t *testing.T) {
	testIncMetric(t, numPoliciesMetric)
}

func TestDecNumPolicies(t *testing.T) {
	testDecMetric(t, numPoliciesMetric)
}

func TestResetNumPolicies(t *testing.T) {
	testResetMetric(t, numPoliciesMetric)
}
