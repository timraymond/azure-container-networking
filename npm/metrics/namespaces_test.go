package metrics

import "testing"

func TestRecordNamespaceApplyTime(t *testing.T) {
	testStopAndRecordApplyTime(t, &applyMetric{
		RecordNamespaceApplyTime,
		GetNamespaceApplyCount,
	})
}
