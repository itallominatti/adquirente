package grpcclient

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type TLSFiles struct {
	CertFile string // certificado deste serviço
	KeyFile  string // chave privada deste serviço
	CAFile   string // autoridade que assina os certificados internos
}

func LoadClientTLS(f TLSFiles) (*tls.Config, error) {
	if f.CertFile == "" {
		return nil, nil
	}
	cert, err := tls.LoadX509KeyPair(f.CertFile, f.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("carregar certificado do cliente: %w", err)
	}
	ca, err := os.ReadFile(f.CAFile)
	if err != nil {
		return nil, fmt.Errorf("ler CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		return nil, errors.New("CA inválida")
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}, RootCAs: pool, MinVersion: tls.VersionTLS13}, nil
}

func LoadServerTLS(f TLSFiles) (*tls.Config, error) {
	cfg, err := LoadClientTLS(f)
	if err != nil || cfg == nil {
		return cfg, err
	}
	cfg.ClientCAs = cfg.RootCAs
	cfg.ClientAuth = tls.RequireAndVerifyClientCert
	return cfg, nil
}

func DialCredentials(cfg *tls.Config) grpc.DialOption {
	if cfg == nil {
		return grpc.WithTransportCredentials(insecure.NewCredentials())
	}
	return grpc.WithTransportCredentials(credentials.NewTLS(cfg))
}
