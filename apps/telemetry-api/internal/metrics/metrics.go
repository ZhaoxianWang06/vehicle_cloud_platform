package metrics

import "github.com/prometheus/client_golang/prometheus"

// Metrics 是车况服务的业务指标。HTTP 次数不放在这里，避免把探针流量算进上报 QPS。
type Metrics struct {
	IngestTotal    *prometheus.CounterVec
	IngestDuration prometheus.Histogram
	OnlineVehicles prometheus.Gauge
}

func New() *Metrics {
	m := &Metrics{
		IngestTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vehicle_telemetry_ingest_total",
			Help: "车况上报次数，按 result 区分 ok、invalid、db_error、cache_error。",
		}, []string{"result"}),
		IngestDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "vehicle_telemetry_ingest_duration_seconds",
			Help:    "车况上报处理耗时。",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2},
		}),
		OnlineVehicles: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "vehicle_telemetry_online_vehicles",
			Help: "最近 60 秒有心跳的车辆数。",
		}),
	}
	prometheus.MustRegister(m.IngestTotal, m.IngestDuration, m.OnlineVehicles)
	return m
}
