package grpcclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	vaultv1 "github.com/itallominatti/adquirente/gen/vault/v1"
	"github.com/itallominatti/adquirente/internal/domain/shared"
	"github.com/itallominatti/adquirente/internal/domain/transaction"
)

type VaultClient struct {
	conn   *grpc.ClientConn
	client vaultv1.VaultServiceClient
}

var _ transaction.CardVault = (*VaultClient)(nil)

func NewVaultClient(addr string, tlsCfg *tls.Config) (*VaultClient, error) {
	conn, err := grpc.NewClient(addr, DialCredentials(tlsCfg), grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	if err != nil {
		return nil, fmt.Errorf("conectar ao vault: %w", err)
	}
	return &VaultClient{conn: conn, client: vaultv1.NewVaultServiceClient(conn)}, nil
}

func (c *VaultClient) Detokenize(ctx context.Context, token string) (transaction.CardData, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	resp, err := c.client.Detokenize(ctx, &vaultv1.DetokenizeRequest{Token: token})
	if status.Code(err) == codes.NotFound {
		return transaction.CardData{}, transaction.ErrInvalidToken
	}
	if err != nil {
		return transaction.CardData{}, err
	}
	return transaction.CardData{PAN: transaction.PAN(resp.Pan), Expiry: resp.Expiry}, nil
}

type TokenizedCard struct {
	Token, Brand, BIN, Last4 string
}

func (c *VaultClient) Tokenize(ctx context.Context, pan, expiry string) (TokenizedCard, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	resp, err := c.client.Tokenize(ctx, &vaultv1.TokenizeRequest{Pan: pan, Expiry: expiry})
	if status.Code(err) == codes.InvalidArgument {
		return TokenizedCard{}, transaction.ErrInvalidPAN
	}
	if err != nil {
		return TokenizedCard{}, fmt.Errorf("%w: %v", shared.ErrNotFound, err)
	}
	return TokenizedCard{Token: resp.Token, Brand: resp.Brand, BIN: resp.Bin, Last4: resp.Last4}, nil
}

func (c *VaultClient) Close() error { return c.conn.Close() }
