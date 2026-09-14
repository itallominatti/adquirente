package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	vaultv1 "github.com/itallominatti/adquirente/gen/vault/v1"
	grpchandler "github.com/itallominatti/adquirente/internal/handler/grpc"
	"github.com/itallominatti/adquirente/internal/infrastructure/config"
	"github.com/itallominatti/adquirente/internal/infrastructure/cryptoutil"
	"github.com/itallominatti/adquirente/internal/infrastructure/grpcclient"
	"github.com/itallominatti/adquirente/internal/infrastructure/observability"
	"github.com/itallominatti/adquirente/internal/infrastructure/postgres"
)

func main() {
	cfg := config.Load()
	log := observability.NewLogger(cfg.LogLevel, "vault")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := observability.SetupTracing(ctx, "vault", cfg.OTLPEndpoint)
	must(log, err, "tracing")

	cipher, err := cryptoutil.NewAESGCM(cfg.VaultKeyBase64)
	must(log, err, "VAULT_KEY_BASE64 (gere com: openssl rand -base64 32)")

	db, err := postgres.Connect(ctx, cfg.DatabaseURL)
	must(log, err, "banco")
	defer db.Close()

	opts := []grpc.ServerOption{grpc.StatsHandler(otelgrpc.NewServerHandler())}
	tlsCfg, err := grpcclient.LoadServerTLS(grpcclient.TLSFiles{CertFile: cfg.TLSCertFile, KeyFile: cfg.TLSKeyFile, CAFile: cfg.TLSCAFile})
	must(log, err, "tls")
	if tlsCfg != nil {
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlsCfg)))
	} else if cfg.IsProd() {
		log.Error("vault em produção exige mTLS")
		os.Exit(1)
	}

	srv := grpc.NewServer(opts...)
	vaultv1.RegisterVaultServiceServer(srv, grpchandler.NewVaultServer(postgres.NewCardVaultStore(db), cipher, log))

	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	must(log, err, "listen")
	go func() {
		log.Info("vault ouvindo", "addr", cfg.GRPCAddr, "mtls", tlsCfg != nil)
		if err := srv.Serve(lis); err != nil {
			log.Error("gRPC caiu", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	srv.GracefulStop()
	_ = shutdownTracing(context.Background())
}

func must(log *slog.Logger, err error, what string) {
	if err != nil {
		log.Error(what, "err", err)
		os.Exit(1)
	}
}
