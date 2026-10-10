package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"vehicle-cloud/control-api/internal/domain"
	"vehicle-cloud/control-api/internal/metrics"
	"vehicle-cloud/platform"
)

type fakeRepo struct {
	cmd    domain.Command
	replay bool
	err    error
}

func (f *fakeRepo) Insert(context.Context, domain.Command) (domain.Command, bool, error) {
	if f.err != nil {
		return domain.Command{}, false, f.err
	}
	if f.cmd.ID == "" {
		f.cmd.ID = "01234567-89ab-4def-8123-456789abcdef"
		f.cmd.Status = domain.StatusPending
	}
	return f.cmd, f.replay, nil
}
func (f *fakeRepo) MarkDispatched(_ context.Context, id string) error {
	if f.cmd.ID == id {
		f.cmd.Status = domain.StatusDispatched
	}
	return nil
}
func (f *fakeRepo) Ack(context.Context, string, string, bool) (domain.Command, error) {
	return f.cmd, nil
}
func (f *fakeRepo) Get(context.Context, string) (domain.Command, error) { return f.cmd, nil }
func (f *fakeRepo) ListByVIN(context.Context, string, int) ([]domain.Command, error) {
	return []domain.Command{f.cmd}, nil
}
func (f *fakeRepo) Ping(context.Context) error { return nil }

type fakeQueue struct {
	err   error
	items map[string]domain.Command
}

func (f *fakeQueue) Enqueue(_ context.Context, cmd domain.Command) error {
	if f.err != nil {
		return f.err
	}
	if f.items == nil {
		f.items = map[string]domain.Command{}
	}
	cmd.Status = domain.StatusDispatched
	f.items[cmd.ID] = cmd
	return nil
}
func (f *fakeQueue) PutStatus(_ context.Context, cmd domain.Command) error {
	if f.items == nil {
		f.items = map[string]domain.Command{}
	}
	f.items[cmd.ID] = cmd
	return nil
}
func (f *fakeQueue) Get(_ context.Context, id string) (domain.Command, bool, error) {
	cmd, ok := f.items[id]
	return cmd, ok, nil
}
func (f *fakeQueue) Ping(context.Context) error { return nil }

var registered *metrics.Metrics

func metricsOnce() *metrics.Metrics {
	if registered == nil {
		registered = metrics.New()
	}
	return registered
}

func newMux(repo *fakeRepo, queue *fakeQueue) http.Handler {
	h := New(repo, queue, platform.NewLogger("control-test"), metricsOnce(), 2*time.Second)
	mux := http.NewServeMux()
	h.Register(mux)
	return mux
}

func TestCreateRejectsUnknownCommand(t *testing.T) {
	mux := newMux(&fakeRepo{}, &fakeQueue{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/commands", strings.NewReader(
		`{"vin":"LSVAA4182E2123456","command":"explode","idempotency_key":"req-0001-lock"}`,
	))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
}

func TestCreateQueuesCommand(t *testing.T) {
	repo := &fakeRepo{cmd: domain.Command{
		ID: "01234567-89ab-4def-8123-456789abcdef", VIN: "LSVAA4182E2123456",
		Name: "door_lock", Status: domain.StatusPending, IdempotencyKey: "req-0001-lock",
	}}
	queue := &fakeQueue{}
	mux := newMux(repo, queue)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/commands", strings.NewReader(
		`{"vin":"LSVAA4182E2123456","command":"door_lock","idempotency_key":"req-0001-lock"}`,
	))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if _, ok := queue.items[repo.cmd.ID]; !ok {
		t.Fatal("command was not enqueued")
	}
}

func TestQueueFailureKeepsAccepted(t *testing.T) {
	repo := &fakeRepo{cmd: domain.Command{
		ID: "01234567-89ab-4def-8123-456789abcdef", Name: "horn", Status: domain.StatusPending,
	}}
	mux := newMux(repo, &fakeQueue{err: errors.New("redis down")})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/commands", strings.NewReader(
		`{"vin":"LSVAA4182E2123456","command":"horn","idempotency_key":"req-0002-horn"}`,
	))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), `"queued":false`) {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
}
