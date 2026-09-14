package settlement

import (
	"context"
	"time"
)

type Repository interface {
	Create(ctx context.Context, s *Settlement) error
	Update(ctx context.Context, s *Settlement) error
	FindByID(ctx context.Context, merchantID, id string) (*Settlement, error)
	ListByMerchant(ctx context.Context, merchantID string, limit int) ([]*Settlement, error)
	ListByDate(ctx context.Context, day time.Time) ([]*Settlement, error)
}

type PaymentGateway interface {
	// Send envia as ordens e devolve uma referência externa (número do arquivo/lote).
	Send(ctx context.Context, s *Settlement) (externalRef string, err error)
}
