package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"vehicle-cloud/platform"
	"vehicle-cloud/telemetry-api/internal/cache"
	"vehicle-cloud/telemetry-api/internal/config"
	"vehicle-cloud/telemetry-api/internal/httpapi"
	"vehicle-cloud/telemetry-api/internal/metrics"
	"vehicle-cloud/telemetry-api/internal/store"
)

func main() {
	log := platform.NewLogger("telemetry-api")
	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := store.OpenMySQL(cfg.MySQLDSN)
	if err != nil {
		log.Error("mysql open", "err", err)
		os.Exit(1)
	}
	defer db.Close()
	rdb := cache.Open(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB, cfg.RedisTLS)
	defer rdb.Close()

	if err := waitReady(ctx, "mysql", db.Ping); err != nil {
		log.Error("mysql wait", "err", err)
		os.Exit(1)
	}
	if err := waitReady(ctx, "redis", rdb.Ping); err != nil {
		log.Error("redis wait", "err", err)
		os.Exit(1)
	}

	m := metrics.New()
	go refreshOnline(ctx, rdb, m, log)

	mux := http.NewServeMux()
	httpapi.New(db, rdb, log, m, cfg.RequestLimit).Register(mux)
	handler := platform.Recover(log, platform.AccessLog(log, mux))

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shut, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(shut); err != nil {
			log.Error("shutdown", "err", err)
		}
	}()

	log.Info("listening", "addr", cfg.HTTPAddr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("listen", "err", err)
		os.Exit(1)
	}
}

func refreshOnline(ctx context.Context, rdb *cache.Redis, m *metrics.Metrics, log platform.Logger) {
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			n, err := rdb.OnlineCount(ctx)
			if err != nil {
				log.Error("online count", "err", err)
				continue
			}
			m.OnlineVehicles.Set(float64(n))
		}
	}
}

func waitReady(ctx context.Context, name string, ping func(context.Context) error) error {
	var err error
	for i := 0; i < 30; i++ {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err = ping(pingCtx)
		cancel()
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return err
}
