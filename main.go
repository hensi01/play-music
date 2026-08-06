package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"

	"play-music/internal/artwork"
	"play-music/internal/auth"
	"play-music/internal/config"
	"play-music/internal/db"
	"play-music/internal/lyrics"
	"play-music/internal/metadata"
	"play-music/internal/scanner"
	"play-music/internal/server"
	"play-music/internal/storage"
	"play-music/internal/store"
	"play-music/internal/stream"
)

func main() {
	cfg := config.Load()
	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// --- PostgreSQL (the only supported database) ---
	if cfg.DatabaseURL == "" {
		logger.Error("DATABASE_URL não configurada (PostgreSQL é obrigatório)")
		os.Exit(1)
	}
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("falha ao conectar no postgres", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool, logger); err != nil {
		logger.Error("falha ao aplicar migrations", "err", err)
		os.Exit(1)
	}
	logger.Info("postgres pronto")

	st := store.New(pool)

	// --- Auth (JWT secret persists in the database) ---
	authSvc, err := auth.New(ctx, cfg, st)
	if err != nil {
		logger.Error("falha ao inicializar auth", "err", err)
		os.Exit(1)
	}

	// --- MinIO/S3 ---
	strg, err := storage.New(cfg.S3)
	if err != nil {
		logger.Error("falha ao inicializar o storage S3", "err", err)
		os.Exit(1)
	}

	metadata.SetFFmpegPath(cfg.FfmpegPath)

	sc := scanner.New(cfg, strg, st, logger)
	streamSvc, err := stream.New(cfg, strg, logger)
	if err != nil {
		logger.Error("falha ao inicializar stream", "err", err)
		os.Exit(1)
	}
	artSvc, err := artwork.New(cfg, st, logger)
	if err != nil {
		logger.Error("falha ao inicializar artwork", "err", err)
		os.Exit(1)
	}
	lyrSvc := lyrics.New(st, strg, logger)

	srv := server.New(server.Dependencies{
		Config:  cfg,
		Store:   st,
		Auth:    authSvc,
		Stream:  streamSvc,
		Storage: strg,
		Artwork: artSvc,
		Lyrics:  lyrSvc,
		Scanner: sc,
		Log:     logger,
	})

	// --- HTTP server ---
	httpSrv := &http.Server{
		Addr:              cfg.ListenAddr(),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		logger.Info("play music backend iniciado", "addr", cfg.ListenAddr(), "version", "1.0.0")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("servidor http", "err", err)
			stop()
		}
	}()

	// --- Initial scan (background) ---
	go func() {
		logger.Info("varredura inicial iniciada")
		sc.Run(context.Background())
	}()

	// --- Scheduled scans (ND_SCANNER_SCHEDULE, e.g. "@every 1h") ---
	if cfg.ScannerSchedule != "" {
		c := cron.New()
		if _, err := c.AddFunc(cfg.ScannerSchedule, func() {
			logger.Info("varredura agendada")
			sc.Run(context.Background())
		}); err != nil {
			logger.Warn("schedule inválido, varredura apenas inicial", "schedule", cfg.ScannerSchedule, "err", err)
		} else {
			c.Start()
			logger.Info("varredura agendada", "schedule", cfg.ScannerSchedule)
		}
	}

	<-ctx.Done()
	logger.Info("encerrando…")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	httpSrv.Shutdown(shutdownCtx)
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
