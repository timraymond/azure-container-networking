package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog"
)

// Timer is a one-time-use tool for recording time between a start and end point
type Timer struct {
	before int64
	after  int64
}

// StartNewTimer creates a new Timer
func StartNewTimer() *Timer {
	return &Timer{time.Now().UnixNano(), 0}
}

// stopAndRecord ends a timer and records its delta in a summary
func (timer *Timer) stopAndRecord(observer prometheus.Summary) {
	observer.Observe(timer.timeElapsed())
}

func (timer *Timer) stopAndRecordCRUDExecTime(observer *prometheus.SummaryVec, op OperationKind) {
	timer.stop()
	if !op.isValid() {
		klog.Errorf("Unknown operation [%v] when recording exec time", op)
		return
	}
	labels := prometheus.Labels{operationLabel: string(op)}
	observer.With(labels).Observe(timer.timeElapsed())
}

func (timer *Timer) stop() {
	timer.after = time.Now().UnixNano()
}

func (timer *Timer) timeElapsed() float64 {
	if timer.after == 0 {
		timer.stop()
	}
	millisecondDifference := float64(timer.after-timer.before) / 1000000.0
	return millisecondDifference
}
