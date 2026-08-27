package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"itam/internal/auth"
	"itam/internal/config"
	"itam/internal/events"
	apihttp "itam/internal/http"
	"itam/internal/metadata"
	"itam/internal/rbac"
	"itam/internal/scheduler"
	"itam/internal/settings"
	"itam/internal/store"
)

func main() {
	cfg := config.Load()

	level := slog.LevelInfo
	if cfg.Debug {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(log)

	db, err := store.NewDB(cfg.DatabaseURL, cfg.Debug)
	if err != nil {
		log.Error("database connection failed", "err", err)
		os.Exit(1)
	}
	defer db.Close()
	log.Info("connected to postgres")

	rdb, err := store.NewRedis(cfg.RedisAddr, cfg.RedisPass)
	if err != nil {
		log.Warn("valkey unavailable, RBAC cache disabled", "err", err)
		rdb = nil
	} else {
		log.Info("connected to valkey")
		defer rdb.Close()
	}

	var bus *events.Bus
	var consumers *events.Consumers
	bus, err = events.Connect(cfg.NATSURL)
	if err != nil {
		log.Warn("NATS unavailable, events disabled", "err", err)
		bus = nil
	} else {
		log.Info("connected to NATS JetStream")
		defer bus.Close()
		consumers, err = events.Start(context.Background(), bus, db, log)
		if err != nil {
			log.Error("failed to start consumers", "err", err)
		}
	}

	// Metadata-driven scheduler: runs due checks and emits events that the rules
	// engine turns into notifications. Skipped when eventing is unavailable.
	schedCtx, schedCancel := context.WithCancel(context.Background())
	defer schedCancel()
	if bus != nil {
		go scheduler.New(db, bus, log).Run(schedCtx)
		log.Info("scheduler started")
	}

	// DB-backed settings (Vault-encrypted secrets) override env where present.
	settingsSvc := settings.New(db, rdb)
	resolveSettings(&cfg, settingsSvc, log)

	metaSvc := metadata.New(db)
	rbacSvc := rbac.New(db, rdb)
	authSvc := auth.New(cfg, db, log)

	// Best-effort admin bootstrap in GoTrue.
	go authSvc.BootstrapAdmin(context.Background())

	srv := apihttp.NewServer(cfg, db, rdb, bus, metaSvc, rbacSvc, authSvc, settingsSvc, log)

	// Generic pull-connector loop (Graph/Arc): authenticates + pulls on schedule,
	// then feeds the same metadata mapping/reconcile pipeline as push ingestion.
	go srv.StartPullLoop(schedCtx)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("http server listening", "addr", cfg.HTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("http server error", "err", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Info("shutting down")

	if consumers != nil {
		consumers.Stop()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(ctx)
}

// resolveSettings overlays DB/Vault-backed settings onto the env-derived config,
// keeping env values as the fallback (so the app boots even if Vault is absent).
func resolveSettings(cfg *config.Config, s *settings.Service, log *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cfg.S3Endpoint = s.GetOr(ctx, "s3_endpoint", cfg.S3Endpoint)
	cfg.S3Region = s.GetOr(ctx, "s3_region", cfg.S3Region)
	cfg.S3Bucket = s.GetOr(ctx, "s3_bucket", cfg.S3Bucket)
	cfg.S3AccessKey = s.GetOr(ctx, "s3_access_key", cfg.S3AccessKey)
	cfg.S3SecretKey = s.GetOr(ctx, "s3_secret_key", cfg.S3SecretKey)
	log.Info("settings resolved", "s3_endpoint", cfg.S3Endpoint, "s3_bucket", cfg.S3Bucket)
}
