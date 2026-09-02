package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"kpi-schedule-bot/server/internal/api"
	"kpi-schedule-bot/server/internal/bot"
	"kpi-schedule-bot/server/internal/campus"
	"kpi-schedule-bot/server/internal/config"
	"kpi-schedule-bot/server/internal/storage"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	if err := storage.Migrate(cfg.DatabasePath); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	db, err := storage.Open(ctx, cfg.DatabasePath)
	cancel()
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()

	campusClient := campus.NewClient(db)
	svc := api.NewService(db, campusClient)

	router := api.NewRouter(svc, cfg.InternalAPIToken)

	var tgBot *bot.Bot
	if cfg.TelegramBotToken != "" {
		tgBot, err = bot.New(cfg.TelegramBotToken, svc, db)
		if err != nil {
			log.Fatalf("bot: %v", err)
		}
		if err := tgBot.StartPolling(); err != nil {
			log.Fatalf("bot polling: %v", err)
		}
		slog.Info("telegram bot started (long polling)")
	} else {
		slog.Info("TELEGRAM_BOT_TOKEN not set — telegram bot disabled")
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("starting server", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	slog.Info("shutting down")
	if tgBot != nil {
		tgBot.Stop()
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
	}
}
