package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/itplus/pushsdk-gateway/internal/config"
	"github.com/itplus/pushsdk-gateway/internal/httpapi"
	"github.com/itplus/pushsdk-gateway/internal/monitor"
	"github.com/itplus/pushsdk-gateway/internal/pushsdk"
	"github.com/itplus/pushsdk-gateway/internal/store"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(logger); err != nil {
		logger.Error("gateway stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	data, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer data.Close()
	if err := data.Migrate(ctx, cfg.MigrationsDir); err != nil {
		return err
	}
	backfilled, err := data.BackfillAccessEventProjections(ctx)
	if err != nil {
		return fmt.Errorf("backfill access-event projections: %w", err)
	}
	if backfilled > 0 {
		logger.Info("backfilled access-event projections", "records", backfilled)
	}
	if err := data.SeedConfiguredTerminals(ctx, cfg.Terminals); err != nil {
		return err
	}
	if err := data.ReconcileConfiguredAdministrator(ctx, cfg.AdminUsername, cfg.AdminPassword); err != nil {
		return err
	}
	if err := data.PurgeExpiredSessions(ctx); err != nil {
		return err
	}

	registry := prometheus.NewRegistry()
	registry.MustRegister(prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	metrics := pushsdk.NewMetrics(registry)
	hub := monitor.NewHub()
	push := pushsdk.NewService(cfg, data, hub, metrics, logger)
	if err := push.RestoreSessions(ctx); err != nil {
		return fmt.Errorf("restore PushSDK sessions: %w", err)
	}
	api, err := httpapi.New(cfg, data, hub, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}), logger)
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	api.Register(mux)
	mux.Handle("/iot/", push)
	server := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           securityHeaders(mux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("gateway listening", "address", cfg.ListenAddress, "terminals", len(cfg.Terminals))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()
	select {
	case <-shutdown:
	case err := <-serverErrors:
		return err
	}
	shutdownContext, done := context.WithTimeout(context.Background(), 20*time.Second)
	defer done()
	return server.Shutdown(shutdownContext)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("Referrer-Policy", "same-origin")
		writer.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(writer, request)
	})
}
