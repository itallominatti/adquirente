package grpc

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	issuerv1 "github.com/itallominatti/adquirente/gen/issuer/v1"
)

type IssuerServer struct {
	issuerv1.UnimplementedIssuerServiceServer
	latency time.Duration
}

func NewIssuerServer(latency time.Duration) *IssuerServer { return &IssuerServer{latency: latency} }

func (s *IssuerServer) Authorize(ctx context.Context, req *issuerv1.AuthorizeRequest) (*issuerv1.AuthorizeResponse, error) {
	delay := s.latency
	if strings.HasSuffix(req.Pan, "9999") {
		delay = 5 * time.Second
	}
	select {
	case <-ctx.Done(): // o cliente desistiu (deadline): não adianta responder
		return nil, ctx.Err()
	case <-time.After(delay):
	}
	switch {
	case strings.HasSuffix(req.Pan, "0000"):
		return &issuerv1.AuthorizeResponse{Approved: false, ResponseCode: "05"}, nil
	case req.Amount > 500_000:
		return &issuerv1.AuthorizeResponse{Approved: false, ResponseCode: "51"}, nil
	}
	return &issuerv1.AuthorizeResponse{Approved: true, AuthorizationCode: randomDigits(6), ResponseCode: "00"}, nil
}

func (s *IssuerServer) Reverse(_ context.Context, _ *issuerv1.ReverseRequest) (*issuerv1.ReverseResponse, error) {
	return &issuerv1.ReverseResponse{Accepted: true}, nil
}

func randomDigits(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		d, _ := rand.Int(rand.Reader, big.NewInt(10))
		b.WriteString(fmt.Sprint(d.Int64()))
	}
	return b.String()
}
