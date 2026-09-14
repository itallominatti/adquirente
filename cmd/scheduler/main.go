package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	apprcv "github.com/itallominatti/adquirente/internal/application/receivable"
	"github.com/itallominatti/adquirente/internal/domain/shared"
	"github.com/itallominatti/adquirente/internal/infrastructure/config"
	"github.com/itallominatti/adquirente/internal/infrastructure/gormrepo"
	"github.com/itallominatti/adquirente/internal/infrastructure/kafka"
	"github.com/itallominatti/adquirente/internal/infrastructure/observability"
	"github.com/itallominatti/adquirente/internal/infrastructure/postgres"
	"github.com/itallominatti/adquirente/internal/infrastructure/registry"
)

func main() {
	cfg := config.Load()
	log := observability.NewLogger(cfg.LogLevel, "scheduler")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := observability.SetupTracing(ctx, "scheduler", cfg.OTLPEndpoint)
	must(log, err, "tracing")

	db, err := postgres.Connect(ctx, cfg.DatabaseURL)
	must(log, err, "banco")
	defer db.Close()
	merchants, err := gormrepo.New(db)
	must(log, err, "gorm")
	reg, err := registry.NewFake(cfg.RegistryLiensFile)
	must(log, err, "registradora")

	uc := apprcv.NewScheduler(postgres.NewUnitOfWork(db), merchants, reg, shared.BrazilCalendar{}, shared.RealClock{}, log)

	consumer := kafka.NewConsumer(cfg.KafkaBrokers, "scheduler",
		[]string{"transaction.captured", "transaction.canceled"}, uc.Handle, log)
	log.Info("scheduler consumindo", "brokers", cfg.KafkaBrokers)
	if err := consumer.Run(ctx); err != nil {
		log.Error("consumer parou", "err", err)
	}
	_ = shutdownTracing(context.Background())
}

func must(log *slog.Logger, err error, what string) {
	if err != nil {
		log.Error(what, "err", err)
		os.Exit(1)
	}
}
