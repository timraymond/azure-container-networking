package metrics

import "testing"

var numPoliciesMetric = &basicMetric{ResetNumPolicies, IncNumPolicies, DecNumPolicies, GetNumPolicies}

func TestRecordPolicyApplyTime(t *testing.T) {
	testStopAndRecordApplyTime(t, &applyMetric{
		RecordPolicyApplyTime,
		GetPolicyApplyCount,
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
