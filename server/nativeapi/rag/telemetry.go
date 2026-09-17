package rag

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type ragPrometheusMetrics struct {
	operationDuration *prometheus.HistogramVec
	retries           *prometheus.CounterVec
	resultCount       prometheus.Histogram
	topScore          prometheus.Histogram
}

var getRAGPrometheusMetrics = sync.OnceValue(func() *ragPrometheusMetrics {
	metrics := &ragPrometheusMetrics{
		operationDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "rag_operation_duration_seconds",
			Help:    "RAG operation latency by stage and outcome.",
			Buckets: prometheus.DefBuckets,
		}, []string{"stage", "status"}),
		retries: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "rag_http_retries_total",
			Help: "Transient HTTP retries by RAG backend.",
		}, []string{"backend"}),
		resultCount: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "rag_retrieval_result_count",
			Help:    "Number of results retained after RAG retrieval filtering.",
			Buckets: []float64{0, 1, 2, 5, 10, 20, 50},
		}),
		topScore: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "rag_retrieval_top_score",
			Help:    "Top cosine-similarity score returned by RAG retrieval.",
			Buckets: []float64{0, .25, .5, .6, .7, .8, .9, 1},
		}),
	}
	prometheus.MustRegister(metrics.operationDuration, metrics.retries, metrics.resultCount, metrics.topScore)
	return metrics
})

func observeRAGOperation(stage string, started time.Time, err error) {
	status := "success"
	if err != nil {
		status = "error"
	}
	getRAGPrometheusMetrics().operationDuration.WithLabelValues(stage, status).Observe(time.Since(started).Seconds())
}

func observeRAGRetry(backend string) {
	getRAGPrometheusMetrics().retries.WithLabelValues(backend).Inc()
}

// ObserveRetrievalQuality records low-cardinality signals useful for tuning
// topK and the minimum score without storing user queries or song metadata.
func ObserveRetrievalQuality(results []SongSearchResult) {
	metrics := getRAGPrometheusMetrics()
	metrics.resultCount.Observe(float64(len(results)))
	if len(results) > 0 {
		metrics.topScore.Observe(results[0].Score)
	}
}
