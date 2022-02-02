package metrics

import "github.com/prometheus/client_golang/prometheus"

// RecordNamespaceApplyTime adds an observation of namespace apply time for the specified apply mode.
// The execution time is from the timer's start until now.
func RecordNamespaceApplyTime(timer *Timer, mode ApplyMode) {
	timer.stopAndRecordApplyTime(namespaceApplyTime, mode)
}

// GetPodApplyCount returns the number of observations for namespace apply time for the specified apply mode.
// This function is slow.
func GetNamespaceApplyCount(mode ApplyMode) (int, error) {
	labels := prometheus.Labels{applyModeLabel: string(mode)}
	return getCountValue(namespaceApplyTime.With(labels))
}
