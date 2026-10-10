package metrics

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	Commands   *prometheus.CounterVec
	Acks       *prometheus.CounterVec
	Duration   prometheus.Histogram
	AckLatency prometheus.Histogram
}

func New() *Metrics {
	m := &Metrics{
		Commands: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vehicle_command_total",
			Help: "车控受理次数，result 取 accepted、replay、invalid、db_error、queue_error。",
		}, []string{"command", "result"}),
		Acks: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vehicle_command_ack_total",
			Help: "车端回执次数。",
		}, []string{"result"}),
		Duration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "vehicle_command_duration_seconds",
			Help:    "车控受理耗时。",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2},
		}),
		AckLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "vehicle_command_ack_latency_seconds",
			Help:    "从指令入库到车端回执的耗时。",
			Buckets: []float64{.05, .1, .25, .5, 1, 2, 5, 10, 30},
		}),
	}
	prometheus.MustRegister(m.Commands, m.Acks, m.Duration, m.AckLatency)
	return m
}
