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
	_ "time/tzdata"

	"kpi-schedule-bot/server/internal/alerts"
	"kpi-schedule-bot/server/internal/api"
	"kpi-schedule-bot/server/internal/bot"
	"kpi-schedule-bot/server/internal/campus"
	"kpi-schedule-bot/server/internal/config"
	"kpi-schedule-bot/server/internal/idle"
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

	var tgBot *bot.Bot
	var webhookHandler http.HandlerFunc
	var alertDispatcher *alerts.Dispatcher
	if cfg.TelegramBotToken != "" {
		tgBot, err = bot.New(cfg.TelegramBotToken, svc, db)
		if err != nil {
			log.Fatalf("bot: %v", err)
		}
		if cfg.ExtensionInstallURL != "" {
			tgBot.SetExtensionInstallURL(cfg.ExtensionInstallURL)
		}
		if err := tgBot.RegisterWebhook(cfg.TelegramWebhookURL, cfg.TelegramWebhookSecret); err != nil {
			log.Fatalf("bot webhook: %v", err)
		}
		webhookHandler = tgBot.WebhookHandler()
		alertDispatcher = alerts.NewDispatcher(db, campusClient, tgBot.GBot())
		slog.Info("telegram bot started (webhook)", "url", cfg.TelegramWebhookURL)
	} else {
		slog.Info("TELEGRAM_BOT_TOKEN not set — telegram bot disabled")
	}

	cronHandler := api.NewCronHandler(alertDispatcher, cfg.CronSecret)

	idleWatcher := idle.New(cfg.IdleTimeout, "/healthz")
	defer idleWatcher.Stop()
	if cfg.IdleTimeout > 0 {
		slog.Info("idle shutdown enabled", "timeout", cfg.IdleTimeout)
	} else {
		slog.Info("idle shutdown disabled")
	}

	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	// In local development or non-sleeping deployments, run a 1-minute ticker for alerts
	if cfg.IdleTimeout <= 0 && alertDispatcher != nil {
		go func() {
			ticker := time.NewTicker(1 * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					_, _ = alertDispatcher.Dispatch(appCtx, time.Now())
				case <-appCtx.Done():
					return
				}
			}
		}()
		slog.Info("in-process alerts ticker started (1m interval)")
	}

	router := api.NewRouterWithOpts(svc, cfg.InternalAPIToken, api.RouterOpts{
		TelegramWebhookHandler: webhookHandler,
		CronHandler:            cronHandler,
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           idleWatcher.Middleware(router),
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

	select {
	case sig := <-stop:
		slog.Info("shutting down due to signal", "signal", sig)
	case <-idleWatcher.Done():
		slog.Info("shutting down due to idle timeout", "timeout", cfg.IdleTimeout)
	}

	if tgBot != nil {
		tgBot.Stop()
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
	}
}
