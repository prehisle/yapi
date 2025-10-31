package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	// HTTPRequestsTotal 统计网关接收的 HTTP 请求总数，按照方法、路由与状态码区分。
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_http_requests_total",
			Help: "Total number of HTTP requests processed by the gateway.",
		},
		[]string{"method", "route", "status"},
	)

	// HTTPRequestDuration 记录 HTTP 请求处理耗时，用于 SLI 统计。
	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gateway_http_request_duration_seconds",
			Help:    "Histogram of HTTP request latencies in seconds.",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5},
		},
		[]string{"method", "route"},
	)

	// UpstreamLatency 统计与上游 LLM 服务交互的耗时及结果。
	UpstreamLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gateway_upstream_latency_seconds",
			Help:    "Latency histogram for upstream LLM requests.",
			Buckets: []float64{0.02, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10},
		},
		[]string{"upstream", "outcome", "status"},
	)
)

func init() {
	prometheus.MustRegister(HTTPRequestsTotal, HTTPRequestDuration, UpstreamLatency)
}

// ObserveHTTPRequest 记录一次网关 HTTP 请求的处理情况。
func ObserveHTTPRequest(method, route string, status int, duration time.Duration) {
	statusLabel := strconv.Itoa(status)
	HTTPRequestsTotal.WithLabelValues(method, route, statusLabel).Inc()
	HTTPRequestDuration.WithLabelValues(method, route).Observe(duration.Seconds())
}

// ObserveUpstream 记录一次与上游的调用时长与结果。
func ObserveUpstream(upstream string, status int, duration time.Duration, hasError bool) {
	statusLabel := strconv.Itoa(status)
	outcome := "success"
	if hasError || status >= 500 || status == 0 {
		outcome = "error"
	}
	UpstreamLatency.WithLabelValues(upstream, outcome, statusLabel).Observe(duration.Seconds())
}
