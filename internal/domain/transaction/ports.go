package transaction

import (
	"context"

	"github.com/itallominatti/adquirente/internal/domain/shared"
)

type AuthorizationRequest struct {
	TransactionID string
	PAN           PAN
	Expiry        string // "MM/YY"
	Amount        shared.Money
	Product       shared.Product
	Installments  int
	MerchantMCC   string
}

type AuthorizationResponse struct {
	Approved          bool
	AuthorizationCode string
	ResponseCode      string // "00" aprovada; "51" saldo insuficiente; "05" não honrar; "91" emissor indisponível
}

type IssuerGateway interface {
	Authorize(ctx context.Context, req AuthorizationRequest) (AuthorizationResponse, error)
	Reverse(ctx context.Context, transactionID string) error
}

type CardData struct {
	PAN    PAN
	Expiry string
}

type CardVault interface {
	Detokenize(ctx context.Context, token string) (CardData, error)
}

type NSUGenerator interface {
	Next(ctx context.Context) (string, error)
}

type Repository interface {
	Create(ctx context.Context, t *Transaction) error
	FindByID(ctx context.Context, merchantID, id string) (*Transaction, error)
	FindByIDForUpdate(ctx context.Context, merchantID, id string) (*Transaction, error)
	Update(ctx context.Context, t *Transaction) error
}
