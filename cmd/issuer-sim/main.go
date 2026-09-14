package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	issuerv1 "github.com/itallominatti/adquirente/gen/issuer/v1"
	grpchandler "github.com/itallominatti/adquirente/internal/handler/grpc"
	"github.com/itallominatti/adquirente/internal/infrastructure/config"
	"github.com/itallominatti/adquirente/internal/infrastructure/grpcclient"
	"github.com/itallominatti/adquirente/internal/infrastructure/observability"
)

func main() {
	cfg := config.Load()
	log := observability.NewLogger(cfg.LogLevel, "issuer-sim")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := observability.SetupTracing(ctx, "issuer-sim", cfg.OTLPEndpoint)
	must(log, err, "tracing")

	opts := []grpc.ServerOption{grpc.StatsHandler(otelgrpc.NewServerHandler())}
	tlsCfg, err := grpcclient.LoadServerTLS(grpcclient.TLSFiles{CertFile: cfg.TLSCertFile, KeyFile: cfg.TLSKeyFile, CAFile: cfg.TLSCAFile})
	must(log, err, "tls")
	if tlsCfg != nil {
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlsCfg))) // mTLS: só clientes com certificado da nossa CA
	}

	srv := grpc.NewServer(opts...)
	issuerv1.RegisterIssuerServiceServer(srv, grpchandler.NewIssuerServer(time.Duration(cfg.IssuerSimLatencyMs)*time.Millisecond))

	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	must(log, err, "listen")
	go func() {
		log.Info("issuer-sim ouvindo", "addr", cfg.GRPCAddr, "latency_ms", cfg.IssuerSimLatencyMs, "mtls", tlsCfg != nil)
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
