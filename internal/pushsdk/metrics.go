package pushsdk

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	Requests       *prometheus.CounterVec
	EventsReceived *prometheus.CounterVec
	EventsAccepted prometheus.Counter
	EventsRejected prometheus.Counter
	SessionsActive prometheus.Gauge
}

func NewMetrics(registerer prometheus.Registerer) *Metrics {
	metrics := &Metrics{
		Requests:       prometheus.NewCounterVec(prometheus.CounterOpts{Name: "pushsdk_requests_total", Help: "PushSDK requests processed by action and outcome."}, []string{"action", "outcome"}),
		EventsReceived: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "pushsdk_events_received_total", Help: "Vendor events received by data format."}, []string{"format"}),
		EventsAccepted: prometheus.NewCounter(prometheus.CounterOpts{Name: "pushsdk_attendance_events_accepted_total", Help: "Attendance events durably persisted."}),
		EventsRejected: prometheus.NewCounter(prometheus.CounterOpts{Name: "pushsdk_events_rejected_total", Help: "Event batches rejected for protocol or persistence errors."}),
		SessionsActive: prometheus.NewGauge(prometheus.GaugeOpts{Name: "pushsdk_sessions_active", Help: "Authenticated PushSDK terminal sessions in this gateway process."}),
	}
	registerer.MustRegister(metrics.Requests, metrics.EventsReceived, metrics.EventsAccepted, metrics.EventsRejected, metrics.SessionsActive)
	return metrics
}
