package main

import (
	"context"
	"embed"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tgs-bot/internal/bot"
	"tgs-bot/internal/config"
	appserver "tgs-bot/internal/server"
	"tgs-bot/internal/store"
)

//go:embed web/*
var webFS embed.FS

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("config", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database", "error", err)
		os.Exit(1)
	}
	defer st.Close()
	api := appserver.New(cfg, st, logger)
	if me, e := api.Telegram.GetMe(ctx); e == nil && me.Username != "" {
		api.BotUsername = me.Username
		logger.Info("telegram bot ready", "username", me.Username)
	} else if e != nil {
		logger.Warn("telegram getMe", "error", e)
	}
	go bot.New(cfg, st, api.Telegram, logger).Run(ctx)
	httpServer := &http.Server{Addr: cfg.ListenAddr, Handler: api.Handler(webFS), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 70 * time.Second, IdleTimeout: 90 * time.Second}
	go func() {
		logger.Info("http server started", "addr", cfg.ListenAddr)
		if e := httpServer.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
			logger.Error("http server", "error", e)
			stop()
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	logger.Info("shutdown complete")
}
