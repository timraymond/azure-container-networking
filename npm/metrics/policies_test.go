package metrics

import "testing"

var numPoliciesMetric = &basicMetric{ResetNumPolicies, IncNumPolicies, DecNumPolicies, GetNumPolicies}

func TestRecordControllerPolicyExecTime(t *testing.T) {
	// copied and modified from testStopAndRecordCRUDExecTime
	for _, mode := range []OperationKind{CreateOp, UpdateOp, DeleteOp} {
		for _, hadError := range []bool{true, false} {
			if mode == CreateOp && hadError {
				continue
			}
			testStopAndRecord(t, &recordingMetric{
				func(timer *Timer) {
					RecordControllerPolicyExecTime(timer, mode, hadError)
				},
				func() (int, error) {
					return GetControllerPolicyExecCount(mode, hadError)
				},
			})
		}
	}
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
