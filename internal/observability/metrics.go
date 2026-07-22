package observability

import "sync"

// MetricsRecorder is the small metric surface used by business packages.
type MetricsRecorder interface {
	IncCounter(name, help string)
	IncCounterWithLabels(name, help string, labels map[string]string)
}

var metricsState struct {
	sync.RWMutex
	recorder MetricsRecorder
}

// SetMetricsRecorder connects business events to the configured metrics backend.
func SetMetricsRecorder(recorder MetricsRecorder) {
	metricsState.Lock()
	metricsState.recorder = recorder
	metricsState.Unlock()
}

// IncCounter records an unlabeled business event when metrics are configured.
func IncCounter(name, help string) {
	metricsState.RLock()
	recorder := metricsState.recorder
	metricsState.RUnlock()
	if recorder != nil {
		recorder.IncCounter(name, help)
	}
}

// IncCounterWithLabels records a labeled business event when metrics are configured.
func IncCounterWithLabels(name, help string, labels map[string]string) {
	metricsState.RLock()
	recorder := metricsState.recorder
	metricsState.RUnlock()
	if recorder != nil {
		recorder.IncCounterWithLabels(name, help, labels)
	}
}
