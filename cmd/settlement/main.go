package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	appstl "github.com/itallominatti/adquirente/internal/application/settlement"
	"github.com/itallominatti/adquirente/internal/domain/shared"
	"github.com/itallominatti/adquirente/internal/infrastructure/bank"
	"github.com/itallominatti/adquirente/internal/infrastructure/config"
	"github.com/itallominatti/adquirente/internal/infrastructure/gormrepo"
	"github.com/itallominatti/adquirente/internal/infrastructure/kafka"
	"github.com/itallominatti/adquirente/internal/infrastructure/observability"
	"github.com/itallominatti/adquirente/internal/infrastructure/postgres"
	"github.com/itallominatti/adquirente/internal/infrastructure/redis"
	"github.com/itallominatti/adquirente/internal/infrastructure/registry"
)

func main() {
	cfg := config.Load()
	log := observability.NewLogger(cfg.LogLevel, "settlement")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := observability.SetupTracing(ctx, "settlement", cfg.OTLPEndpoint)
	must(log, err, "tracing")
	defer func() { _ = shutdownTracing(context.Background()) }()

	db, err := postgres.Connect(ctx, cfg.DatabaseURL)
	must(log, err, "banco")
	defer db.Close()
	merchants, err := gormrepo.New(db)
	must(log, err, "gorm")
	reg, err := registry.NewFake(cfg.RegistryLiensFile)
	must(log, err, "registradora")
	gateway, err := bank.NewFileGateway(cfg.SettlementOutputDir)
	must(log, err, "banco liquidante")
	lock := redis.NewLock(cfg.RedisAddr)
	must(log, lock.Ping(ctx), "redis")
	defer lock.Close()

	day := time.Now().UTC()
	if cfg.SettlementDate != "" {
		day, err = time.Parse("2006-01-02", cfg.SettlementDate)
		must(log, err, "SETTLEMENT_DATE deve ser YYYY-MM-DD")
	}

	uc := appstl.NewSettleDue(postgres.NewUnitOfWork(db), merchants, reg, gateway, shared.BrazilCalendar{}, shared.RealClock{}, lock, 500, log)
	rep, err := uc.Execute(ctx, day)
	if err != nil {
		log.Error("liquidação falhou", "date", day.Format("2006-01-02"), "err", err)
		os.Exit(1)
	}
	log.Info("liquidação concluída", "date", rep.Date.Format("2006-01-02"), "skipped", rep.Skipped, "merchants", rep.Merchants,
		"settlements", rep.Settlements, "failed", rep.Failed, "paid", rep.Paid.String(), "retained", rep.Retained.String())

	// Publica os eventos gerados (settlement.completed/failed) antes de sair: o worker não tem relay permanente.
	pub := kafka.NewPublisher(cfg.KafkaBrokers)
	defer pub.Close()
	relay := kafka.NewRelay(db, pub, log, time.Second, 100)
	relayCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	relay.Run(relayCtx)
}

func must(log *slog.Logger, err error, what string) {
	if err != nil {
		log.Error(what, "err", err)
		os.Exit(1)
	}
}
