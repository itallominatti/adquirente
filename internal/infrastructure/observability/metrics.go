package observability

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	registry *prometheus.Registry

	HTTPRequests   *prometheus.CounterVec   // route, method, status
	HTTPDuration   *prometheus.HistogramVec // route, method
	Authorizations *prometheus.CounterVec   // product, result (approved|denied|error)
	IssuerLatency  prometheus.Histogram
	SettledAmount  prometheus.Counter // centavos liquidados (soma)
	OutboxLag      prometheus.Gauge
}

func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	m := &Metrics{
		registry:     reg,
		HTTPRequests: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "http_requests_total", Help: "Requisições HTTP"}, []string{"route", "method", "status"}),
		HTTPDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "http_request_duration_seconds", Help: "Duração das requisições",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5}}, []string{"route", "method"}),
		Authorizations: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "authorizations_total", Help: "Autorizações por produto e resultado"}, []string{"product", "result"}),
		IssuerLatency: prometheus.NewHistogram(prometheus.HistogramOpts{Name: "issuer_authorize_duration_seconds", Help: "Tempo de resposta do emissor",
			Buckets: []float64{.05, .1, .25, .5, 1, 2, 5}}),
		SettledAmount: prometheus.NewCounter(prometheus.CounterOpts{Name: "settled_amount_cents_total", Help: "Total liquidado (centavos)"}),
		OutboxLag:     prometheus.NewGauge(prometheus.GaugeOpts{Name: "outbox_pending_events", Help: "Eventos ainda não publicados"}),
	}
	reg.MustRegister(m.HTTPRequests, m.HTTPDuration, m.Authorizations, m.IssuerLatency, m.SettledAmount, m.OutboxLag)
	return m
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
