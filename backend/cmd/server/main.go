package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bookshelf/internal/auth"
	"bookshelf/internal/config"
	"bookshelf/internal/database"
	"bookshelf/internal/proxy"
	"bookshelf/internal/server"
	"bookshelf/internal/settings"
	"bookshelf/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	store, err := storage.New(cfg.DataDir)
	if err != nil {
		slog.Error("initialize storage", "error", err)
		os.Exit(1)
	}
	var secretSource config.SessionSecretSource
	cfg.SessionSecret, secretSource, err = config.ResolveSessionSecret(store.Root(), cfg.SessionSecret)
	if err != nil {
		slog.Error("initialize session secret", "error", err)
		os.Exit(1)
	}
	switch secretSource {
	case config.SessionSecretCreated:
		slog.Info("session secret created")
	case config.SessionSecretLoaded:
		slog.Info("session secret loaded")
	case config.SessionSecretEnvironment:
		slog.Info("session secret configured from environment")
	}
	db, err := database.Open(store.Root(), cfg.SQLiteBusyTimeoutMS)
	if err != nil {
		slog.Error("initialize database", "error", err)
		os.Exit(1)
	}
	resolver, err := proxy.New(cfg.TrustedProxies)
	if err != nil {
		slog.Error("invalid trusted proxy configuration", "error", err)
		os.Exit(1)
	}
	settingsService := settings.New(db, settings.Defaults{Enabled: cfg.OPDSEnabled, AccessMode: cfg.OPDSAccessMode, Username: cfg.OPDSUsername, Password: cfg.OPDSPassword, PublicBaseURL: cfg.PublicBaseURL})
	if err := settingsService.EnsureExistingInstall(); err != nil {
		slog.Error("initialize system settings", "error", err)
		os.Exit(1)
	}
	authService := auth.New(db, cfg.SessionTTL)
	httpServer := &http.Server{Addr: ":" + cfg.Port, Handler: server.New(cfg, db, store, authService, settingsService, resolver), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		slog.Info("bookshelf server started", "port", cfg.Port, "environment", cfg.Environment)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
	}
}
