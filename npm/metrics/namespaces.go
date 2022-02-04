package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// RecordNamespaceExecTime adds an observation of namespace exec time for the specified operation.
// The execution time is from the timer's start until now.
func RecordNamespaceExecTime(timer *Timer, op OperationKind) {
	timer.stopAndRecordCRUDExecTime(controllerNamespaceExecTime, op)
}

// GetNamespaceExecCount returns the number of observations for namespace exec time for the specified operation.
// This function is slow.
func GetNamespaceExecCount(op OperationKind) (int, error) {
	labels := prometheus.Labels{operationLabel: string(op)}
	return getCountVecValue(controllerNamespaceExecTime, labels)
}
