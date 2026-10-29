package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"infobot/internal/bot"
	"infobot/internal/config"
	"infobot/internal/vault"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration", "err", err)
		os.Exit(1)
	}

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func(c context.Context, l *slog.Logger) {
		if sErr := startServer(c, l); sErr != nil {
			log.Error("server", "err", sErr)
		}
	}(ctx, log)

	syncer := vault.NewSyncer(cfg.VaultURL, cfg.VaultBranch, cfg.VaultDir, cfg.GitToken)
	svc := vault.NewService(syncer, vault.Dirs{Images: cfg.ImagesDir})

	syncCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	_, syncErr := svc.Refresh(syncCtx)
	cancel()
	if syncErr != nil {
		log.Warn("initial git sync failed", "err", syncErr)
		if !vault.HasNotes(cfg.VaultDir) {
			log.Error("no vault available: sync failed and there are no local notes", "dir", cfg.VaultDir)
			os.Exit(1)
		}
		if err := svc.LoadIndex(); err != nil {
			log.Error("build index from the local clone", "err", err)
			os.Exit(1)
		}
	}

	if idx := svc.Index(); idx != nil {
		log.Info("vault indexed",
			"teachers", len(idx.Teachers),
			"subjects", len(idx.Subjects),
			"notes", len(idx.Notes),
			"dir", cfg.VaultDir,
		)
	}

	go syncLoop(ctx, log, svc, cfg.SyncInterval)

	app, err := bot.New(ctx, cfg, svc, log)
	if err != nil {
		log.Error("start telegram bot", "err", err)
		os.Exit(1)
	}

	log.Info("bot started")
	app.Run(ctx)
	log.Info("shutdown complete")
}

// startServer starts a minimal HTTP server for health check.
func startServer(ctx context.Context, logger *slog.Logger) error {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	server := &http.Server{
		Addr: ":" + port,
	}
	h := http.NewServeMux()
	logger.Info("starting server", "port", port)
	h.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	server.Handler = h

	if sErr := server.ListenAndServe(); sErr != nil && !errors.Is(sErr, http.ErrServerClosed) {
		return sErr
	}

	<-ctx.Done()

	logger.Info("stopping server...")
	serverCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return server.Shutdown(serverCtx)
}

func syncLoop(ctx context.Context, log *slog.Logger, svc *vault.Service, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			syncCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			updated, err := svc.Refresh(syncCtx)
			cancel()
			if err != nil {
				log.Warn("scheduled sync failed", "err", err)
				continue
			}
			idx := svc.Index()
			log.Info("scheduled sync",
				"updated", updated,
				"teachers", len(idx.Teachers),
				"subjects", len(idx.Subjects),
			)
		}
	}
}
