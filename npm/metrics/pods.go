package metrics

import "github.com/prometheus/client_golang/prometheus"

// RecordPolicyApplyTime adds an observation of pod apply time for the specified apply mode.
// The execution time is from the timer's start until now.
func RecordPodApplyTime(timer *Timer, mode ApplyMode) {
	timer.stopAndRecordApplyTime(podApplyTime, mode)
}

// GetPodApplyCount returns the number of observations for pod apply time for the specified apply mode.
// This function is slow.
func GetPodApplyCount(mode ApplyMode) (int, error) {
	labels := prometheus.Labels{applyModeLabel: string(mode)}
	return getCountValue(podApplyTime.With(labels))
}
