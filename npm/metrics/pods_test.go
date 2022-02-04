package metrics

import "testing"

func TestRecordPodExecTime(t *testing.T) {
	testStopAndRecordCRUDExecTime(t, &crudExecMetric{
		RecordPodExecTime,
		GetPodExecCount,
	})
}
