package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"vehicle-cloud/control-api/internal/cache"
	"vehicle-cloud/control-api/internal/config"
	"vehicle-cloud/control-api/internal/httpapi"
	"vehicle-cloud/control-api/internal/metrics"
	"vehicle-cloud/control-api/internal/store"
	"vehicle-cloud/platform"
)

func main() {
	log := platform.NewLogger("control-api")
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

	if err := waitReady(ctx, db.Ping); err != nil {
		log.Error("mysql wait", "err", err)
		os.Exit(1)
	}
	if err := waitReady(ctx, rdb.Ping); err != nil {
		log.Error("redis wait", "err", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	httpapi.New(db, rdb, log, metrics.New(), cfg.RequestLimit).Register(mux)
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           platform.Recover(log, platform.AccessLog(log, mux)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shut, cancel := context.WithTimeout(context.Background(), 20*time.Second)
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

func waitReady(ctx context.Context, ping func(context.Context) error) error {
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
