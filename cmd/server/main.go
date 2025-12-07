package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/quueli/mc-server-monitor/internal/config"
	"github.com/quueli/mc-server-monitor/internal/httpapi"
	"github.com/quueli/mc-server-monitor/internal/monitor"
	"github.com/quueli/mc-server-monitor/internal/store"
)

func main() {
	cfg := config.Load()
	setupLogger(cfg.LogLevel)

	st, closeStore, err := openStore(cfg)
	if err != nil {
		slog.Error("open store", "err", err)
		os.Exit(1)
	}
	if closeStore != nil {
		defer closeStore()
	}

	regCtx := context.Background()
	for _, e := range cfg.Servers {
		if _, err := st.UpsertServer(regCtx, e.Host, e.Port); err != nil {
			slog.Error("register server", "host", e.Host, "err", err)
		}
	}

	poller := monitor.NewPoller(st, monitor.Options{
		Interval:    cfg.PollInterval,
		Concurrency: cfg.Concurrency,
		PingTimeout: cfg.PingTimeout,
		CacheTTL:    cfg.CacheTTL,
		Retention:   cfg.Retention,
	})

	pollCtx, stopPoller := context.WithCancel(context.Background())
	defer stopPoller()
	go poller.Run(pollCtx)

	api := httpapi.NewAPI(st, poller)
	srv := httpapi.NewServer(cfg.HTTPAddr, api.Handler())

	go func() {
		slog.Info("http server listening", "addr", cfg.HTTPAddr)
		if err := srv.Start(); err != nil {
			slog.Error("http server", "err", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutting down")

	stopPoller()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown", "err", err)
	}
}

func openStore(cfg config.Config) (store.Store, func(), error) {
	if cfg.DBDSN == "" {
		return store.NewMemStore(), nil, nil
	}
	s, err := store.NewSQLStore(cfg.DBDriver, cfg.DBDSN)
	if err != nil {
		return nil, nil, err
	}
	return s, func() { _ = s.Close() }, nil
}

func setupLogger(level string) {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})))
}
