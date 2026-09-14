package anticipation

import "context"

type Repository interface {
	Create(ctx context.Context, a *Anticipation) error
	FindByID(ctx context.Context, merchantID, id string) (*Anticipation, error)
	ListByMerchant(ctx context.Context, merchantID string, limit int) ([]*Anticipation, error)
}
