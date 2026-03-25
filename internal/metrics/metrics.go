package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	ProbeUp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_probe_up",
			Help: "Whether the latest probe succeeded",
		},
		[]string{"node_id", "server", "port", "phase"},
	)

	ProbeDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "node_probe_duration_seconds",
			Help:    "Probe duration seconds by phase",
			Buckets: []float64{0.1, 0.3, 0.5, 1, 2, 3, 5, 8, 15},
		},
		[]string{"node_id", "server", "port", "phase"},
	)

	ProbeTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "node_probe_total",
			Help: "Total number of probes",
		},
		[]string{"node_id", "server", "port", "status", "phase", "error_type"},
	)

	LastSuccess = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_probe_last_success_timestamp_seconds",
			Help: "Last successful probe timestamp",
		},
		[]string{"node_id", "server", "port"},
	)

	HTTPStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_probe_http_status_code",
			Help: "HTTP status code from latest probe",
		},
		[]string{"node_id", "server", "port", "probe_class"},
	)

	ActiveNodes = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "node_source_active_nodes",
			Help: "Current active nodes loaded from config plus subscription",
		},
	)

	SourceRefreshTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "node_source_refresh_total",
			Help: "Total source refresh attempts",
		},
		[]string{"source", "status"},
	)

	NodeInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_probe_info",
			Help: "Static node metadata for joining in dashboards",
		},
		[]string{"node_id", "name", "source", "server", "port", "server_name", "utls_fingerprint"},
	)
)

func MustRegister() {
	prometheus.MustRegister(
		ProbeUp,
		ProbeDuration,
		ProbeTotal,
		LastSuccess,
		HTTPStatus,
		ActiveNodes,
		SourceRefreshTotal,
		NodeInfo,
	)
}
