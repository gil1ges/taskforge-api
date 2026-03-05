package observability

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	RequestsTotal  *prometheus.CounterVec
	RequestLatency *prometheus.HistogramVec
	ErrorsTotal    *prometheus.CounterVec
}

func NewMetrics() *Metrics {
	return &Metrics{
		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "http_requests_total", Help: "Total HTTP requests"},
			[]string{"method", "path", "status"},
		),
		RequestLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{Name: "http_request_latency_seconds", Help: "HTTP latency (seconds)"},
			[]string{"method", "path"},
		),
		ErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "http_errors_total", Help: "Total HTTP errors"},
			[]string{"path", "status"},
		),
	}
}
