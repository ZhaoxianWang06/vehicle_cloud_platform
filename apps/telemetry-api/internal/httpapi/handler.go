package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"vehicle-cloud/platform"
	"vehicle-cloud/telemetry-api/internal/domain"
	"vehicle-cloud/telemetry-api/internal/metrics"
)

type Repository interface {
	SaveTelemetry(ctx context.Context, rec domain.Record) error
	Latest(ctx context.Context, vin string) (domain.Record, error)
	History(ctx context.Context, vin string, limit int) ([]domain.Record, error)
	Ping(ctx context.Context) error
}

type Cache interface {
	PutLatest(ctx context.Context, rec domain.Record) error
	GetLatest(ctx context.Context, vin string) (domain.Record, bool, error)
	Ping(ctx context.Context) error
}

type Handler struct {
	repo         Repository
	cache        Cache
	log          platform.Logger
	metrics      *metrics.Metrics
	requestLimit time.Duration
}

func New(repo Repository, cache Cache, log platform.Logger, m *metrics.Metrics, limit time.Duration) *Handler {
	return &Handler{repo: repo, cache: cache, log: log, metrics: m, requestLimit: limit}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", h.healthz)
	mux.HandleFunc("GET /readyz", h.readyz)
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("POST /api/v1/telemetry", h.ingest)
	mux.HandleFunc("GET /api/v1/vehicles/{vin}/history", h.history)
	mux.HandleFunc("GET /api/v1/vehicles/{vin}", h.latest)
}

func (h *Handler) healthz(w http.ResponseWriter, _ *http.Request) {
	platform.WriteJSON(w, http.StatusOK, 0, "ok", map[string]string{"service": "telemetry-api"})
}

// readyz 只检查 MySQL。Redis 故障时服务降级为回源数据库，不应该被负载均衡摘掉。
func (h *Handler) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	if err := h.repo.Ping(ctx); err != nil {
		h.log.Error("mysql not ready", "err", err)
		platform.WriteJSON(w, http.StatusServiceUnavailable, 50001, "mysql not ready", nil)
		return
	}
	platform.WriteJSON(w, http.StatusOK, 0, "ok", nil)
}

func (h *Handler) ingest(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	result := "ok"
	defer func() {
		h.metrics.IngestDuration.Observe(time.Since(start).Seconds())
		h.metrics.IngestTotal.WithLabelValues(result).Inc()
	}()

	ctx, cancel := context.WithTimeout(r.Context(), h.requestLimit)
	defer cancel()

	var in domain.Record
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(&in); err != nil {
		result = "invalid"
		platform.WriteJSON(w, http.StatusBadRequest, 40001, "请求体不是合法 JSON", nil)
		return
	}
	rec, err := domain.Normalize(in)
	if err != nil {
		result = "invalid"
		platform.WriteJSON(w, http.StatusBadRequest, 40002, err.Error(), nil)
		return
	}
	if err := h.repo.SaveTelemetry(ctx, rec); err != nil {
		result = "db_error"
		h.log.Error("save telemetry", "err", err, "vin", rec.VIN)
		platform.WriteJSON(w, http.StatusServiceUnavailable, 50001, "车况写入失败", nil)
		return
	}
	if err := h.cache.PutLatest(ctx, rec); err != nil {
		result = "cache_error"
		h.log.Error("cache telemetry", "err", err, "vin", rec.VIN)
		platform.WriteJSON(w, http.StatusOK, 0, "ok", map[string]any{"vin": rec.VIN, "cached": false})
		return
	}
	platform.WriteJSON(w, http.StatusOK, 0, "ok", map[string]any{"vin": rec.VIN, "cached": true})
}

func (h *Handler) latest(w http.ResponseWriter, r *http.Request) {
	vin, ok := h.vin(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.requestLimit)
	defer cancel()

	if rec, hit, err := h.cache.GetLatest(ctx, vin); err != nil {
		h.log.Error("read cache", "err", err, "vin", vin)
	} else if hit {
		platform.WriteJSON(w, http.StatusOK, 0, "ok", domain.Latest{Record: rec, Source: "redis"})
		return
	}

	rec, err := h.repo.Latest(ctx, vin)
	if errors.Is(err, domain.ErrNotFound) {
		platform.WriteJSON(w, http.StatusNotFound, 40401, "没有该车辆的车况", nil)
		return
	}
	if err != nil {
		h.log.Error("read latest", "err", err, "vin", vin)
		platform.WriteJSON(w, http.StatusServiceUnavailable, 50001, "读取车况失败", nil)
		return
	}
	_ = h.cache.PutLatest(ctx, rec)
	platform.WriteJSON(w, http.StatusOK, 0, "ok", domain.Latest{Record: rec, Source: "mysql"})
}

func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	vin, ok := h.vin(w, r)
	if !ok {
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 500 {
			platform.WriteJSON(w, http.StatusBadRequest, 40003, "limit 必须在 1 到 500 之间", nil)
			return
		}
		limit = n
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.requestLimit)
	defer cancel()
	rows, err := h.repo.History(ctx, vin, limit)
	if err != nil {
		h.log.Error("read history", "err", err, "vin", vin)
		platform.WriteJSON(w, http.StatusServiceUnavailable, 50001, "读取历史失败", nil)
		return
	}
	platform.WriteJSON(w, http.StatusOK, 0, "ok", map[string]any{"vin": vin, "items": rows})
}

func (h *Handler) vin(w http.ResponseWriter, r *http.Request) (string, bool) {
	vin := r.PathValue("vin")
	if err := platform.ValidateVIN(vin); err != nil {
		platform.WriteJSON(w, http.StatusBadRequest, 40002, err.Error(), nil)
		return "", false
	}
	return vin, true
}
