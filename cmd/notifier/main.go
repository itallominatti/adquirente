package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/itallominatti/adquirente/internal/application/notification"
	"github.com/itallominatti/adquirente/internal/infrastructure/config"
	"github.com/itallominatti/adquirente/internal/infrastructure/gormrepo"
	"github.com/itallominatti/adquirente/internal/infrastructure/kafka"
	"github.com/itallominatti/adquirente/internal/infrastructure/observability"
	"github.com/itallominatti/adquirente/internal/infrastructure/postgres"
	"github.com/itallominatti/adquirente/internal/infrastructure/webhook"
)

func main() {
	cfg := config.Load()
	log := observability.NewLogger(cfg.LogLevel, "notifier")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := postgres.Connect(ctx, cfg.DatabaseURL)
	must(log, err, "banco")
	defer db.Close()
	merchants, err := gormrepo.New(db)
	must(log, err, "gorm")

	uc := notification.NewNotifier(merchants, webhook.NewHTTPSender(), cfg.WebhookSecret, log)
	topics := []string{
		"transaction.authorized", "transaction.captured", "transaction.canceled", "transaction.chargebacked",
		"receivable.scheduled", "receivable.anticipated", "settlement.completed", "settlement.failed", "chargeback.opened",
	}
	consumer := kafka.NewConsumer(cfg.KafkaBrokers, "notifier", topics, uc.Handle, log)
	log.Info("notifier consumindo", "topics", topics)
	if err := consumer.Run(ctx); err != nil {
		log.Error("consumer parou", "err", err)
	}
}

func must(log *slog.Logger, err error, what string) {
	if err != nil {
		log.Error(what, "err", err)
		os.Exit(1)
	}
}
