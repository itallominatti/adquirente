package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/itallominatti/adquirente/internal/domain/anticipation"
	"github.com/itallominatti/adquirente/internal/domain/shared"
)

type AnticipationRepository struct{ db DBTX }

func NewAnticipationRepository(db DBTX) *AnticipationRepository {
	return &AnticipationRepository{db: db}
}

var _ anticipation.Repository = (*AnticipationRepository)(nil)

const antColumns = `id, merchant_id, items, gross_amount, discount_amount, net_amount, rate_bps_month, requested_at, status, version`

func (r *AnticipationRepository) Create(ctx context.Context, a *anticipation.Anticipation) error {
	items, _ := json.Marshal(a.Items())
	_, err := r.db.ExecContext(ctx, `INSERT INTO anticipations (`+antColumns+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		a.ID(), a.MerchantID(), items, int64(a.Gross()), int64(a.Discount()), int64(a.Net()), int(a.RateMonth()), a.RequestedAt(), string(a.Status()), a.Version())
	if err != nil {
		return fmt.Errorf("inserir antecipação: %w", err)
	}
	return nil
}

func (r *AnticipationRepository) FindByID(ctx context.Context, merchantID, id string) (*anticipation.Anticipation, error) {
	list, err := r.list(ctx, `SELECT `+antColumns+` FROM anticipations WHERE id = $1 AND merchant_id = $2`, id, merchantID)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, shared.ErrNotFound
	}
	return list[0], nil
}

func (r *AnticipationRepository) ListByMerchant(ctx context.Context, merchantID string, limit int) ([]*anticipation.Anticipation, error) {
	return r.list(ctx, `SELECT `+antColumns+` FROM anticipations WHERE merchant_id = $1 ORDER BY requested_at DESC LIMIT $2`, merchantID, limit)
}

func (r *AnticipationRepository) list(ctx context.Context, query string, args ...any) ([]*anticipation.Anticipation, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listar antecipações: %w", err)
	}
	defer rows.Close()
	var out []*anticipation.Anticipation
	for rows.Next() {
		var (
			id, merchantID, status string
			itemsJSON              []byte
			gross, discount, net   int64
			rate, version          int
			requestedAt            time.Time
		)
		if err := rows.Scan(&id, &merchantID, &itemsJSON, &gross, &discount, &net, &rate, &requestedAt, &status, &version); err != nil {
			return nil, fmt.Errorf("ler antecipação: %w", err)
		}
		var items []anticipation.Item
		if err := json.Unmarshal(itemsJSON, &items); err != nil {
			return nil, err
		}
		out = append(out, anticipation.Restore(id, merchantID, items, shared.Money(gross), shared.Money(discount), shared.Money(net),
			shared.Bps(rate), requestedAt, anticipation.Status(status), version))
	}
	return out, rows.Err()
}
