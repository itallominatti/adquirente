package grpcclient

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"time"

	"github.com/sony/gobreaker/v2"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	issuerv1 "github.com/itallominatti/adquirente/gen/issuer/v1"
	"github.com/itallominatti/adquirente/internal/domain/shared"
	"github.com/itallominatti/adquirente/internal/domain/transaction"
)

type IssuerClient struct {
	conn    *grpc.ClientConn
	client  issuerv1.IssuerServiceClient
	breaker *gobreaker.CircuitBreaker[*issuerv1.AuthorizeResponse]
}

var _ transaction.IssuerGateway = (*IssuerClient)(nil)

func NewIssuerClient(addr string, tlsCfg *tls.Config) (*IssuerClient, error) {
	conn, err := grpc.NewClient(addr, DialCredentials(tlsCfg), grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	if err != nil {
		return nil, fmt.Errorf("conectar ao emissor: %w", err)
	}
	// Disjuntor: abre após 5 falhas consecutivas; fica aberto 10s; depois testa (meio-aberto).
	// Negativas (Approved=false) NÃO contam como falha: só erro técnico.
	settings := gobreaker.Settings{
		Name:        "issuer",
		Timeout:     10 * time.Second,
		ReadyToTrip: func(c gobreaker.Counts) bool { return c.ConsecutiveFailures >= 5 },
	}
	return &IssuerClient{conn: conn, client: issuerv1.NewIssuerServiceClient(conn), breaker: gobreaker.NewCircuitBreaker[*issuerv1.AuthorizeResponse](settings)}, nil
}

func (c *IssuerClient) Authorize(ctx context.Context, req transaction.AuthorizationRequest) (transaction.AuthorizationResponse, error) {
	product := issuerv1.Product_PRODUCT_CREDIT
	if req.Product == shared.Debit {
		product = issuerv1.Product_PRODUCT_DEBIT
	}
	resp, err := c.breaker.Execute(func() (*issuerv1.AuthorizeResponse, error) {
		return c.client.Authorize(ctx, &issuerv1.AuthorizeRequest{
			TransactionId: req.TransactionID, Pan: string(req.PAN), Expiry: req.Expiry, Amount: int64(req.Amount),
			Currency: "BRL", Product: product, Installments: int32(req.Installments), MerchantMcc: req.MerchantMCC,
		})
	})
	if err != nil {
		return transaction.AuthorizationResponse{}, translate(err)
	}
	return transaction.AuthorizationResponse{Approved: resp.Approved, AuthorizationCode: resp.AuthorizationCode, ResponseCode: resp.ResponseCode}, nil
}

func (c *IssuerClient) Reverse(ctx context.Context, transactionID string) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := c.client.Reverse(ctx, &issuerv1.ReverseRequest{TransactionId: transactionID})
	return translate(err)
}

func (c *IssuerClient) Close() error { return c.conn.Close() }

func translate(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
		return fmt.Errorf("circuit breaker aberto: %w", err)
	}
	switch status.Code(err) {
	case codes.DeadlineExceeded:
		return fmt.Errorf("%w: %v", context.DeadlineExceeded, err)
	case codes.Unavailable:
		return fmt.Errorf("emissor indisponível: %w", err)
	}
	return err
}
