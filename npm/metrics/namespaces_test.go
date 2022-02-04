package metrics

import "testing"

func TestRecordNamespaceExecTime(t *testing.T) {
	testStopAndRecordCRUDExecTime(t, &crudExecMetric{
		RecordNamespaceExecTime,
		GetNamespaceExecCount,
	})
}
