package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	BotActions = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Name:      "bot_actions_total",
		Help:      "Total number of bot actions",
	}, []string{"action"})

	DeletedMessages = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Name:      "deleted_messages_total",
		Help:      "Total number of deleted messages",
	}, []string{"reason"})

	UpdateProcessingDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: Namespace,
		Name:      "update_processing_duration_seconds",
		Help:      "Duration of update processing",
		Buckets:   prometheus.DefBuckets,
	}, []string{"type", "status"})
	ActiveMutes = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      "active_mutes",
		Help:      "Number of currently active mutes",
	})
)

func IncBotAction(action string) {
	BotActions.WithLabelValues(action).Inc()
}

func IncDeletedMessages(reason string) {
	DeletedMessages.WithLabelValues(reason).Inc()
}

func SetActiveMutes(count float64) {
	ActiveMutes.Set(count)
}

func ObserveUpdateProcessing(updateType string, duration float64, err error) {
	status := "success"
	if err != nil {
		status = "error"
	}
	UpdateProcessingDuration.WithLabelValues(updateType, status).Observe(duration)
}
