package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// ChargeTotal counts total charge attempts by provider and status.
	ChargeTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "payment_charge_total",
		Help: "Total number of charge attempts",
	}, []string{"provider", "currency", "status"})

	// ChargeLatency tracks charge request latency by provider.
	ChargeLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "payment_charge_duration_seconds",
		Help:    "Charge request latency in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"provider"})

	// WebhookTotal counts incoming webhooks by provider and event type.
	WebhookTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "payment_webhook_total",
		Help: "Total number of webhooks received",
	}, []string{"provider", "event_type"})

	// ProviderCircuitState tracks circuit breaker state per provider.
	// 0 = closed (healthy), 1 = half-open, 2 = open (unhealthy)
	ProviderCircuitState = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "payment_provider_circuit_state",
		Help: "Circuit breaker state per provider (0=closed, 1=half-open, 2=open)",
	}, []string{"provider"})

	// ReconciliationDiscrepancies tracks discrepancies found per provider.
	ReconciliationDiscrepancies = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "payment_reconciliation_discrepancies_total",
		Help: "Total reconciliation discrepancies found",
	}, []string{"provider"})

	// HTTPRequestDuration tracks HTTP request latency by method and path.
	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path", "status"})
)

// Handler returns the Prometheus HTTP handler for /metrics endpoint.
func Handler() http.Handler {
	return promhttp.Handler()
}
