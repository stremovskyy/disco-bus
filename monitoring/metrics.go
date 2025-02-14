package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// MessagePublished tracks the number of messages published
	MessagePublished = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "disco_bus_messages_published_total",
		Help: "The total number of messages published",
	}, []string{"topic"})

	// MessageProcessed tracks the number of messages processed by handlers
	MessageProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "disco_bus_messages_processed_total",
		Help: "The total number of messages processed",
	}, []string{"topic", "status"})

	// ProcessingDuration tracks message processing duration
	ProcessingDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "disco_bus_message_processing_duration_seconds",
		Help:    "Time spent processing messages",
		Buckets: prometheus.DefBuckets,
	}, []string{"topic"})

	// LockOperations tracks lock operations
	LockOperations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "disco_bus_lock_operations_total",
		Help: "The total number of lock operations",
	}, []string{"operation", "status"})

	// ActiveSubscriptions tracks number of active subscriptions
	ActiveSubscriptions = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "disco_bus_active_subscriptions",
		Help: "The number of active subscriptions",
	}, []string{"topic"})
)
