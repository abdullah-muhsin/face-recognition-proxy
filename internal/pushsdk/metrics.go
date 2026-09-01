package pushsdk

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	Requests        *prometheus.CounterVec
	EventsReceived  *prometheus.CounterVec
	EventsPersisted prometheus.Counter
	EventsRejected  prometheus.Counter
	SessionsActive  prometheus.Gauge
}

func NewMetrics(registerer prometheus.Registerer) *Metrics {
	metrics := &Metrics{
		Requests:        prometheus.NewCounterVec(prometheus.CounterOpts{Name: "pushsdk_requests_total", Help: "PushSDK requests processed by action and outcome."}, []string{"action", "outcome"}),
		EventsReceived:  prometheus.NewCounterVec(prometheus.CounterOpts{Name: "pushsdk_events_received_total", Help: "Vendor events received by data format."}, []string{"format"}),
		EventsPersisted: prometheus.NewCounter(prometheus.CounterOpts{Name: "pushsdk_device_events_persisted_total", Help: "Device event payloads durably persisted."}),
		EventsRejected:  prometheus.NewCounter(prometheus.CounterOpts{Name: "pushsdk_events_rejected_total", Help: "Event batches rejected for protocol or persistence errors."}),
		SessionsActive:  prometheus.NewGauge(prometheus.GaugeOpts{Name: "pushsdk_sessions_active", Help: "Authenticated PushSDK terminal sessions in this gateway process."}),
	}
	registerer.MustRegister(metrics.Requests, metrics.EventsReceived, metrics.EventsPersisted, metrics.EventsRejected, metrics.SessionsActive)
	return metrics
}
