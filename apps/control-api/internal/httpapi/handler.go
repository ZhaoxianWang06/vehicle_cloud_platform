package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"vehicle-cloud/control-api/internal/domain"
	"vehicle-cloud/control-api/internal/metrics"
	"vehicle-cloud/platform"
)

type Repository interface {
	Insert(ctx context.Context, cmd domain.Command) (domain.Command, bool, error)
	MarkDispatched(ctx context.Context, id string) error
	Ack(ctx context.Context, id, message string, success bool) (domain.Command, error)
	Get(ctx context.Context, id string) (domain.Command, error)
	ListByVIN(ctx context.Context, vin string, limit int) ([]domain.Command, error)
	Ping(ctx context.Context) error
}

type Queue interface {
	Enqueue(ctx context.Context, cmd domain.Command) error
	PutStatus(ctx context.Context, cmd domain.Command) error
	Get(ctx context.Context, id string) (domain.Command, bool, error)
	Ping(ctx context.Context) error
}

type Handler struct {
	repo         Repository
	queue        Queue
	log          platform.Logger
	metrics      *metrics.Metrics
	requestLimit time.Duration
}

func New(repo Repository, queue Queue, log platform.Logger, m *metrics.Metrics, limit time.Duration) *Handler {
	return &Handler{repo: repo, queue: queue, log: log, metrics: m, requestLimit: limit}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", h.healthz)
	mux.HandleFunc("GET /readyz", h.readyz)
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("POST /api/v1/commands", h.create)
	mux.HandleFunc("POST /api/v1/commands/{id}/ack", h.ack)
	mux.HandleFunc("GET /api/v1/commands/{id}", h.get)
	mux.HandleFunc("GET /api/v1/vehicles/{vin}/commands", h.list)
}

func (h *Handler) healthz(w http.ResponseWriter, _ *http.Request) {
	platform.WriteJSON(w, http.StatusOK, 0, "ok", map[string]string{"service": "control-api"})
}

// readyz 同时检查 MySQL 和 Redis。车控指令必须能落库并且进入下行队列，缺一就从负载均衡摘除。
func (h *Handler) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	if err := h.repo.Ping(ctx); err != nil {
		h.log.Error("mysql not ready", "err", err)
		platform.WriteJSON(w, http.StatusServiceUnavailable, 50001, "mysql not ready", nil)
		return
	}
	if err := h.queue.Ping(ctx); err != nil {
		h.log.Error("redis not ready", "err", err)
		platform.WriteJSON(w, http.StatusServiceUnavailable, 50002, "redis not ready", nil)
		return
	}
	platform.WriteJSON(w, http.StatusOK, 0, "ok", nil)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	command := "unknown"
	result := "accepted"
	defer func() {
		h.metrics.Duration.Observe(time.Since(start).Seconds())
		h.metrics.Commands.WithLabelValues(command, result).Inc()
	}()

	ctx, cancel := context.WithTimeout(r.Context(), h.requestLimit)
	defer cancel()

	var in domain.CreateInput
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(&in); err != nil {
		result = "invalid"
		platform.WriteJSON(w, http.StatusBadRequest, 40001, "请求体不是合法 JSON", nil)
		return
	}
	cmd, err := domain.NormalizeCreate(in)
	if err != nil {
		result = "invalid"
		platform.WriteJSON(w, http.StatusBadRequest, 40002, err.Error(), nil)
		return
	}
	command = cmd.Name

	stored, replay, err := h.repo.Insert(ctx, cmd)
	if err != nil {
		result = "db_error"
		h.log.Error("insert command", "err", err, "vin", cmd.VIN)
		platform.WriteJSON(w, http.StatusServiceUnavailable, 50001, "指令写入失败", nil)
		return
	}
	command = stored.Name

	if stored.Status == domain.StatusPending {
		if err := h.queue.Enqueue(ctx, stored); err != nil {
			result = "queue_error"
			h.log.Error("enqueue command", "err", err, "id", stored.ID)
			platform.WriteJSON(w, http.StatusAccepted, 0, "accepted", map[string]any{
				"id": stored.ID, "status": stored.Status, "queued": false,
			})
			return
		}
		if err := h.repo.MarkDispatched(ctx, stored.ID); err != nil {
			h.log.Error("mark dispatched", "err", err, "id", stored.ID)
		}
		stored.Status = domain.StatusDispatched
	}

	status := http.StatusAccepted
	message := "accepted"
	if replay {
		result = "replay"
		status = http.StatusOK
		message = "idempotent replay"
	}
	platform.WriteJSON(w, status, 0, message, map[string]any{
		"id": stored.ID, "vin": stored.VIN, "command": stored.Name, "status": stored.Status, "queued": stored.Status != domain.StatusPending,
	})
}

func (h *Handler) ack(w http.ResponseWriter, r *http.Request) {
	result := "ok"
	defer func() { h.metrics.Acks.WithLabelValues(result).Inc() }()

	id := r.PathValue("id")
	if !domain.ValidID(id) {
		result = "invalid"
		platform.WriteJSON(w, http.StatusBadRequest, 40002, "指令 id 不合法", nil)
		return
	}
	var in domain.AckInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		result = "invalid"
		platform.WriteJSON(w, http.StatusBadRequest, 40001, "请求体不是合法 JSON", nil)
		return
	}
	if len(in.Message) > 255 {
		result = "invalid"
		platform.WriteJSON(w, http.StatusBadRequest, 40002, "message 最长 255 字符", nil)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.requestLimit)
	defer cancel()
	cmd, err := h.repo.Ack(ctx, id, in.Message, in.Success)
	if errors.Is(err, domain.ErrNotFound) {
		result = "not_found"
		platform.WriteJSON(w, http.StatusNotFound, 40401, "指令不存在", nil)
		return
	}
	if err != nil {
		result = "db_error"
		h.log.Error("ack command", "err", err, "id", id)
		platform.WriteJSON(w, http.StatusServiceUnavailable, 50001, "回执写入失败", nil)
		return
	}
	if !in.Success {
		result = "failed"
	}
	if !cmd.CreatedAt.IsZero() && cmd.AckedAt != nil {
		h.metrics.AckLatency.Observe(cmd.AckedAt.Sub(cmd.CreatedAt).Seconds())
	}
	if err := h.queue.PutStatus(ctx, cmd); err != nil {
		h.log.Error("cache ack", "err", err, "id", id)
	}
	platform.WriteJSON(w, http.StatusOK, 0, "ok", cmd)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !domain.ValidID(id) {
		platform.WriteJSON(w, http.StatusBadRequest, 40002, "指令 id 不合法", nil)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.requestLimit)
	defer cancel()
	if cmd, ok, err := h.queue.Get(ctx, id); err != nil {
		h.log.Error("cache get", "err", err, "id", id)
	} else if ok {
		platform.WriteJSON(w, http.StatusOK, 0, "ok", cmd)
		return
	}
	cmd, err := h.repo.Get(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		platform.WriteJSON(w, http.StatusNotFound, 40401, "指令不存在", nil)
		return
	}
	if err != nil {
		h.log.Error("get command", "err", err, "id", id)
		platform.WriteJSON(w, http.StatusServiceUnavailable, 50001, "读取指令失败", nil)
		return
	}
	_ = h.queue.PutStatus(ctx, cmd)
	platform.WriteJSON(w, http.StatusOK, 0, "ok", cmd)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	vin := r.PathValue("vin")
	if err := platform.ValidateVIN(vin); err != nil {
		platform.WriteJSON(w, http.StatusBadRequest, 40002, err.Error(), nil)
		return
	}
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 100 {
			platform.WriteJSON(w, http.StatusBadRequest, 40003, "limit 必须在 1 到 100 之间", nil)
			return
		}
		limit = n
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.requestLimit)
	defer cancel()
	items, err := h.repo.ListByVIN(ctx, vin, limit)
	if err != nil {
		h.log.Error("list commands", "err", err, "vin", vin)
		platform.WriteJSON(w, http.StatusServiceUnavailable, 50001, "读取指令失败", nil)
		return
	}
	platform.WriteJSON(w, http.StatusOK, 0, "ok", map[string]any{"vin": vin, "items": items})
}
