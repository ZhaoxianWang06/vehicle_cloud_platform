package httpapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"vehicle-cloud/platform"
	"vehicle-cloud/telemetry-api/internal/domain"
	"vehicle-cloud/telemetry-api/internal/metrics"
)

type fakeRepo struct {
	saved []domain.Record
	err   error
}

func (f *fakeRepo) SaveTelemetry(_ context.Context, rec domain.Record) error {
	if f.err != nil {
		return f.err
	}
	f.saved = append(f.saved, rec)
	return nil
}

func (f *fakeRepo) Latest(context.Context, string) (domain.Record, error) {
	if len(f.saved) == 0 {
		return domain.Record{}, domain.ErrNotFound
	}
	return f.saved[len(f.saved)-1], nil
}

func (f *fakeRepo) History(context.Context, string, int) ([]domain.Record, error) {
	return f.saved, nil
}

func (f *fakeRepo) Ping(context.Context) error { return nil }

type fakeCache struct {
	err  error
	data map[string]domain.Record
}

func (f *fakeCache) PutLatest(_ context.Context, rec domain.Record) error {
	if f.err != nil {
		return f.err
	}
	if f.data == nil {
		f.data = map[string]domain.Record{}
	}
	f.data[rec.VIN] = rec
	return nil
}

func (f *fakeCache) GetLatest(_ context.Context, vin string) (domain.Record, bool, error) {
	rec, ok := f.data[vin]
	return rec, ok, nil
}

func (f *fakeCache) Ping(context.Context) error { return nil }

func newTestHandler(repo *fakeRepo, cache *fakeCache) http.Handler {
	h := New(repo, cache, quietLogger(), metricsOnce(), 2*time.Second)
	mux := http.NewServeMux()
	h.Register(mux)
	return mux
}

var registered *metrics.Metrics

func metricsOnce() *metrics.Metrics {
	if registered == nil {
		registered = metrics.New()
	}
	return registered
}

func quietLogger() platform.Logger {
	return platform.NewLogger("telemetry-test")
}

func TestIngestRejectsBadBody(t *testing.T) {
	mux := newTestHandler(&fakeRepo{}, &fakeCache{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/telemetry", strings.NewReader("{"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body %s", rec.Code, rec.Body.String())
	}
}

func TestIngestThenLatestFromCache(t *testing.T) {
	repo := &fakeRepo{}
	cache := &fakeCache{}
	mux := newTestHandler(repo, cache)
	body := `{"vin":"LSVAA4182E2123456","speed_kmh":62.5,"battery_pct":78,"lng":121.47,"lat":31.23,"odometer_km":15230.4,"reported_at":"2026-10-09T08:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/telemetry", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ingest status = %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/vehicles/LSVAA4182E2123456", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("latest status = %d %s", rec.Code, rec.Body.String())
	}
	payload, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(payload), `"source":"redis"`) {
		t.Fatalf("body = %s", payload)
	}
}

func TestCacheFailureStillAccepts(t *testing.T) {
	mux := newTestHandler(&fakeRepo{}, &fakeCache{err: context.DeadlineExceeded})
	body := `{"vin":"LSVAA4182E2123456","speed_kmh":1,"battery_pct":1,"lng":0,"lat":0,"odometer_km":1}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/telemetry", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"cached":false`) {
		t.Fatalf("status = %d body %s", rec.Code, rec.Body.String())
	}
}
