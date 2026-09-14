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

	appant "github.com/itallominatti/adquirente/internal/application/anticipation"
	appcb "github.com/itallominatti/adquirente/internal/application/chargeback"
	appmerchant "github.com/itallominatti/adquirente/internal/application/merchant"
	apprcv "github.com/itallominatti/adquirente/internal/application/receivable"
	apptx "github.com/itallominatti/adquirente/internal/application/transaction"
	"github.com/itallominatti/adquirente/internal/domain/anticipation"
	"github.com/itallominatti/adquirente/internal/domain/shared"
	httphandler "github.com/itallominatti/adquirente/internal/handler/http"
	"github.com/itallominatti/adquirente/internal/handler/http/middleware"
	"github.com/itallominatti/adquirente/internal/infrastructure/config"
	"github.com/itallominatti/adquirente/internal/infrastructure/gormrepo"
	"github.com/itallominatti/adquirente/internal/infrastructure/grpcclient"
	"github.com/itallominatti/adquirente/internal/infrastructure/kafka"
	"github.com/itallominatti/adquirente/internal/infrastructure/observability"
	"github.com/itallominatti/adquirente/internal/infrastructure/postgres"
	"github.com/itallominatti/adquirente/internal/infrastructure/registry"
)

func main() {
	cfg := config.Load()
	log := observability.NewLogger(cfg.LogLevel, "api")

	// ctx é cancelado no SIGTERM (Kubernetes) ou Ctrl+C: é o gatilho do graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := observability.SetupTracing(ctx, "api", cfg.OTLPEndpoint)
	must(log, err, "tracing")

	db, err := postgres.Connect(ctx, cfg.DatabaseURL)
	must(log, err, "banco")
	defer db.Close()

	merchants, err := gormrepo.New(db)
	must(log, err, "gorm")

	tlsCfg, err := grpcclient.LoadClientTLS(grpcclient.TLSFiles{CertFile: cfg.TLSCertFile, KeyFile: cfg.TLSKeyFile, CAFile: cfg.TLSCAFile})
	must(log, err, "tls")
	issuer, err := grpcclient.NewIssuerClient(cfg.IssuerAddr, tlsCfg)
	must(log, err, "emissor")
	defer issuer.Close()
	vault, err := grpcclient.NewVaultClient(cfg.VaultAddr, tlsCfg)
	must(log, err, "vault")
	defer vault.Close()

	reg, err := registry.NewFake(cfg.RegistryLiensFile)
	must(log, err, "registradora")

	publicKey, err := middleware.LoadPublicKey(cfg.JWTPublicKeyFile)
	must(log, err, "chave pública JWT")

	// ---- montagem (Composition Root) ----
	clock := shared.RealClock{}
	uow := postgres.NewUnitOfWork(db)
	read := postgres.NewRepositories(db) // leituras fora de transação
	metrics := observability.NewMetrics()

	authorize := apptx.NewAuthorize(merchants, vault, issuer, uow, clock, cfg.IssuerTimeout,
		apptx.MaxAmountRule(shared.Money(cfg.MaxTxAmount)))
	lifecycle := apptx.NewLifecycle(uow, clock)
	anticipate := appant.NewAnticipate(uow, read.Receivables, merchants, reg, anticipation.CompoundPricer{}, clock, log)

	router := httphandler.NewRouter(httphandler.Deps{
		Log: log, Metrics: metrics, PublicKey: publicKey, JWTIssuer: cfg.JWTIssuer, JWTAudience: cfg.JWTAudience,
		Idempotency:  postgres.NewIdempotencyStore(db, 24*time.Hour),
		Ready:        db.PingContext,
		Merchants:    httphandler.NewMerchantHandler(appmerchant.NewOnboard(merchants, clock), appmerchant.NewReview(merchants, clock), merchants),
		Transactions: httphandler.NewTransactionHandler(authorize, lifecycle, apptx.NewGet(read.Transactions), vault, metrics),
		Receivables:  httphandler.NewReceivableHandler(apprcv.NewQuery(read.Receivables), read.Settlements),
		Anticipation: httphandler.NewAnticipationHandler(anticipate),
		Chargebacks:  httphandler.NewChargebackHandler(appcb.NewOpen(uow, clock)),
		Prod:         cfg.IsProd(),
	})

	// Relay da outbox: publica no Kafka o que os casos de uso gravaram. Roda junto com a API.
	publisher := kafka.NewPublisher(cfg.KafkaBrokers)
	defer publisher.Close()
	go kafka.NewRelay(db, publisher, log, 500*time.Millisecond, 100).Run(ctx)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		log.Info("API ouvindo", "addr", cfg.HTTPAddr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("servidor HTTP caiu", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("desligando: aguardando requisições em andamento")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	_ = shutdownTracing(shutdownCtx)
}

func must(log *slog.Logger, err error, what string) {
	if err != nil {
		log.Error("falha na inicialização", "component", what, "err", err)
		os.Exit(1)
	}
}
