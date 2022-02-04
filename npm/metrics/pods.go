package metrics

import "github.com/prometheus/client_golang/prometheus"

// RecordPodExecTime adds an observation of pod exec time for the specified operation.
// The execution time is from the timer's start until now.
func RecordPodExecTime(timer *Timer, op OperationKind) {
	timer.stopAndRecordCRUDExecTime(controllerPodExecTime, op)
}

// GetPodExecCount returns the number of observations for pod exec time for the specified operation.
// This function is slow.
func GetPodExecCount(op OperationKind) (int, error) {
	labels := prometheus.Labels{operationLabel: string(op)}
	return getCountVecValue(controllerPodExecTime, labels)
}
