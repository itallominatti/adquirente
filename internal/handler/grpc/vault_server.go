package grpc

import (
	"context"
	"errors"
	"log/slog"
	"regexp"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	vaultv1 "github.com/itallominatti/adquirente/gen/vault/v1"
	"github.com/itallominatti/adquirente/internal/domain/shared"
	"github.com/itallominatti/adquirente/internal/domain/transaction"
	"github.com/itallominatti/adquirente/internal/infrastructure/cryptoutil"
	"github.com/itallominatti/adquirente/internal/infrastructure/postgres"
)

type CardStore interface {
	Save(ctx context.Context, rec postgres.CardRecord) error
	Find(ctx context.Context, token string) (postgres.CardRecord, error)
}

type VaultServer struct {
	vaultv1.UnimplementedVaultServiceServer
	store  CardStore
	cipher *cryptoutil.Cipher
	log    *slog.Logger
}

func NewVaultServer(store CardStore, cipher *cryptoutil.Cipher, log *slog.Logger) *VaultServer {
	return &VaultServer{store: store, cipher: cipher, log: log}
}

var expiryRe = regexp.MustCompile(`^(0[1-9]|1[0-2])/\d{2}$`)

func (s *VaultServer) Tokenize(ctx context.Context, req *vaultv1.TokenizeRequest) (*vaultv1.TokenizeResponse, error) {
	if !transaction.Luhn(req.Pan) || !expiryRe.MatchString(req.Expiry) {
		return nil, status.Error(codes.InvalidArgument, "cartão inválido")
	}
	token := shared.NewID("tok")
	info, err := transaction.CardInfoFromPAN(token, req.Pan)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "cartão inválido")
	}
	panCT, err := s.cipher.Encrypt([]byte(req.Pan))
	if err != nil {
		return nil, status.Error(codes.Internal, "falha ao cifrar")
	}
	expCT, err := s.cipher.Encrypt([]byte(req.Expiry))
	if err != nil {
		return nil, status.Error(codes.Internal, "falha ao cifrar")
	}
	if err := s.store.Save(ctx, postgres.CardRecord{Token: token, PANCiphertext: panCT, ExpiryCiphertext: expCT, Brand: info.Brand, BIN: info.BIN, Last4: info.Last4}); err != nil {
		s.log.ErrorContext(ctx, "gravar no cofre", "err", err)
		return nil, status.Error(codes.Internal, "falha ao guardar")
	}
	// Repare: o log só tem token e last4. Nunca o PAN.
	s.log.InfoContext(ctx, "cartão tokenizado", "token", token, "brand", info.Brand, "last4", info.Last4)
	return &vaultv1.TokenizeResponse{Token: token, Brand: info.Brand, Bin: info.BIN, Last4: info.Last4}, nil
}

func (s *VaultServer) Detokenize(ctx context.Context, req *vaultv1.DetokenizeRequest) (*vaultv1.DetokenizeResponse, error) {
	rec, err := s.store.Find(ctx, req.Token)
	if errors.Is(err, shared.ErrNotFound) {
		return nil, status.Error(codes.NotFound, "token desconhecido")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "falha ao ler")
	}
	pan, err := s.cipher.Decrypt(rec.PANCiphertext)
	if err != nil {
		return nil, status.Error(codes.Internal, "falha ao decifrar")
	}
	exp, err := s.cipher.Decrypt(rec.ExpiryCiphertext)
	if err != nil {
		return nil, status.Error(codes.Internal, "falha ao decifrar")
	}
	// Auditoria (PCI): todo acesso a dado de cartão fica registrado, com quem pediu (mTLS identifica o chamador).
	s.log.InfoContext(ctx, "cartão detokenizado", "token", req.Token, "last4", rec.Last4)
	return &vaultv1.DetokenizeResponse{Pan: string(pan), Expiry: string(exp)}, nil
}
