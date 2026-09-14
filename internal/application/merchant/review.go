package merchant

import (
	"context"

	"github.com/itallominatti/adquirente/internal/domain/merchant"
	"github.com/itallominatti/adquirente/internal/domain/shared"
)

type Review struct {
	merchants merchant.Repository
	clock     shared.Clock
}

func NewReview(merchants merchant.Repository, clock shared.Clock) *Review {
	return &Review{merchants: merchants, clock: clock}
}

func (uc *Review) Approve(ctx context.Context, id string) (*merchant.Merchant, error) {
	return uc.change(ctx, id, func(m *merchant.Merchant) error { return m.Approve(uc.clock.Now()) })
}

func (uc *Review) Block(ctx context.Context, id string) (*merchant.Merchant, error) {
	return uc.change(ctx, id, func(m *merchant.Merchant) error { return m.Block(uc.clock.Now()) })
}

func (uc *Review) change(ctx context.Context, id string, apply func(*merchant.Merchant) error) (*merchant.Merchant, error) {
	m, err := uc.merchants.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := apply(m); err != nil {
		return nil, err
	}
	if err := uc.merchants.Update(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

type UpdateSettingsCommand struct {
	MerchantID       string
	WebhookURL       *string // nil = não alterar
	AutoAnticipation *bool
}

func (uc *Review) UpdateSettings(ctx context.Context, cmd UpdateSettingsCommand) (*merchant.Merchant, error) {
	return uc.change(ctx, cmd.MerchantID, func(m *merchant.Merchant) error {
		now := uc.clock.Now()
		if cmd.WebhookURL != nil {
			if err := m.SetWebhook(*cmd.WebhookURL, now); err != nil {
				return err
			}
		}
		if cmd.AutoAnticipation != nil {
			m.EnableAutoAnticipation(*cmd.AutoAnticipation, now)
		}
		return nil
	})
}
