// Package metrics centralizes Prometheus metric definitions.
package metrics

import (
	"github.com/digineo/texd"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	processedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "texd_processed_total",
		Help: "Number of jobs processed, by status",
	}, []string{"status"})

	ProcessedSuccess  = processedTotal.WithLabelValues("success")
	ProcessedFailure  = processedTotal.WithLabelValues("failure")
	ProcessedRejected = processedTotal.WithLabelValues("rejected")
	ProcessedAborted  = processedTotal.WithLabelValues("aborted")

	ProcessingDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "texd_processing_duration_seconds",
		Help: "Overview of processing time per job",
		Buckets: []float64{
			.05, .1, .5, // expected range for errors to occur while processing input files
			1, 1.5, 2, 2.5, 3, 3.5, 4, 4.5, 5, // some jobs are fast
			6, 7, 8, 9, 10, 20, 30, 60, // other jobs might take time
		},
	})

	InputSize = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "texd_input_file_size_bytes",
		Help:    "Overview of input file sizes by category",
		Buckets: prometheus.ExponentialBuckets(512, 2, 13), // 0.5 KiB .. 2 MiB
	}, []string{"type"})

	OutputSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "texd_output_file_size_bytes",
		Help:    "Overview of generated document sizes, success only",
		Buckets: prometheus.ExponentialBuckets(2048, 2, 13), // 2 KiB .. 8 MiB
	})

	JobsQueueLength = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "texd_job_queue_length",
		Help: "Length of rendering queue, i.e. how many documents are waiting for processing",
	})

	JobQueueRatio = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "texd_job_queue_ratio",
		Help: "Queue capacity indicator, with 0 meaning empty and 1 meaning full queue",
	})

	Info = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name:        "texd_info",
		Help:        "Various runtime and configuration information",
		ConstLabels: prometheus.Labels{"version": texd.Version()},
	}, []string{"mode"})
)
