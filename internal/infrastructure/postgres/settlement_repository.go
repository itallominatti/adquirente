package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/itallominatti/adquirente/internal/domain/settlement"
	"github.com/itallominatti/adquirente/internal/domain/shared"
)

type SettlementRepository struct{ db DBTX }

func NewSettlementRepository(db DBTX) *SettlementRepository { return &SettlementRepository{db: db} }

var _ settlement.Repository = (*SettlementRepository)(nil)

const stlColumns = `id, settlement_date, merchant_id, total_amount, to_merchant, to_creditors, retained, orders, receivable_ids,
	status, external_ref, failure_reason, version, created_at, updated_at`

func (r *SettlementRepository) Create(ctx context.Context, s *settlement.Settlement) error {
	orders, _ := json.Marshal(s.Orders())
	ids, _ := json.Marshal(s.ReceivableIDs())
	_, err := r.db.ExecContext(ctx, `INSERT INTO settlements (`+stlColumns+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		s.ID(), s.Date(), s.MerchantID(), int64(s.Total()), int64(s.ToMerchant()), int64(s.ToCreditors()), int64(s.Retained()),
		orders, ids, string(s.Status()), s.ExternalRef(), s.FailureReason(), s.Version(), s.CreatedAt(), s.UpdatedAt())
	if err != nil {
		return fmt.Errorf("inserir liquidação: %w", err)
	}
	return nil
}

func (r *SettlementRepository) Update(ctx context.Context, s *settlement.Settlement) error {
	res, err := r.db.ExecContext(ctx, `UPDATE settlements SET status = $2, external_ref = $3, failure_reason = $4, version = $5, updated_at = $6
		WHERE id = $1 AND version < $5`, s.ID(), string(s.Status()), s.ExternalRef(), s.FailureReason(), s.Version(), s.UpdatedAt())
	if err != nil {
		return fmt.Errorf("atualizar liquidação: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return shared.ErrConcurrentUpdate
	}
	return nil
}

func (r *SettlementRepository) FindByID(ctx context.Context, merchantID, id string) (*settlement.Settlement, error) {
	list, err := r.list(ctx, `SELECT `+stlColumns+` FROM settlements WHERE id = $1 AND merchant_id = $2`, id, merchantID)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, shared.ErrNotFound
	}
	return list[0], nil
}

func (r *SettlementRepository) ListByMerchant(ctx context.Context, merchantID string, limit int) ([]*settlement.Settlement, error) {
	return r.list(ctx, `SELECT `+stlColumns+` FROM settlements WHERE merchant_id = $1 ORDER BY settlement_date DESC LIMIT $2`, merchantID, limit)
}

func (r *SettlementRepository) ListByDate(ctx context.Context, day time.Time) ([]*settlement.Settlement, error) {
	return r.list(ctx, `SELECT `+stlColumns+` FROM settlements WHERE settlement_date = $1 ORDER BY merchant_id`, shared.DateOnly(day))
}

func (r *SettlementRepository) list(ctx context.Context, query string, args ...any) ([]*settlement.Settlement, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listar liquidações: %w", err)
	}
	defer rows.Close()
	var out []*settlement.Settlement
	for rows.Next() {
		var (
			id, merchantID, status, externalRef, failureReason string
			total, toMerchant, toCreditors, retained           int64
			ordersJSON, idsJSON                                []byte
			version                                            int
			date, createdAt, updatedAt                         time.Time
		)
		if err := rows.Scan(&id, &date, &merchantID, &total, &toMerchant, &toCreditors, &retained, &ordersJSON, &idsJSON,
			&status, &externalRef, &failureReason, &version, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("ler liquidação: %w", err)
		}
		var orders []settlement.Order
		var ids []string
		if err := json.Unmarshal(ordersJSON, &orders); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(idsJSON, &ids); err != nil {
			return nil, err
		}
		out = append(out, settlement.Restore(id, shared.DateOnly(date), merchantID, shared.Money(total), shared.Money(toMerchant),
			shared.Money(toCreditors), shared.Money(retained), orders, ids, settlement.Status(status), externalRef, failureReason,
			createdAt, updatedAt, version))
	}
	if err := rows.Err(); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return out, nil
}
