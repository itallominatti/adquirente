package merchant

import "context"

type Repository interface {
	Create(ctx context.Context, m *Merchant) error
	FindByID(ctx context.Context, id string) (*Merchant, error)
	Update(ctx context.Context, m *Merchant) error
}
