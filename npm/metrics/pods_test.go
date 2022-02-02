package metrics

import "testing"

func TestRecordPodApplyTime(t *testing.T) {
	testStopAndRecordApplyTime(t, &applyMetric{
		RecordPodApplyTime,
		GetPodApplyCount,
	})
}
