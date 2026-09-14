package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/itallominatti/adquirente/internal/domain/chargeback"
	"github.com/itallominatti/adquirente/internal/domain/shared"
)

type ChargebackRepository struct{ db DBTX }

func NewChargebackRepository(db DBTX) *ChargebackRepository { return &ChargebackRepository{db: db} }

var _ chargeback.Repository = (*ChargebackRepository)(nil)

func (r *ChargebackRepository) Create(ctx context.Context, c *chargeback.Chargeback) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO chargebacks (id, transaction_id, merchant_id, reason_code, amount, status, opened_at, deadline, resolved_at, version)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		c.ID(), c.TransactionID(), c.MerchantID(), c.ReasonCode(), int64(c.Amount()), string(c.Status()), c.OpenedAt(), c.Deadline(), c.ResolvedAt(), c.Version())
	if err != nil {
		return fmt.Errorf("inserir chargeback: %w", err)
	}
	return nil
}

func (r *ChargebackRepository) Update(ctx context.Context, c *chargeback.Chargeback) error {
	res, err := r.db.ExecContext(ctx, `UPDATE chargebacks SET status = $2, resolved_at = $3, version = $4 WHERE id = $1 AND version < $4`,
		c.ID(), string(c.Status()), c.ResolvedAt(), c.Version())
	if err != nil {
		return fmt.Errorf("atualizar chargeback: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return shared.ErrConcurrentUpdate
	}
	return nil
}

func (r *ChargebackRepository) FindByID(ctx context.Context, merchantID, id string) (*chargeback.Chargeback, error) {
	var (
		cid, txID, mid, reason, status string
		amount                         int64
		openedAt, deadline             time.Time
		resolvedAt                     sql.NullTime
		version                        int
	)
	err := r.db.QueryRowContext(ctx, `SELECT id, transaction_id, merchant_id, reason_code, amount, status, opened_at, deadline, resolved_at, version
		FROM chargebacks WHERE id = $1 AND merchant_id = $2`, id, merchantID).
		Scan(&cid, &txID, &mid, &reason, &amount, &status, &openedAt, &deadline, &resolvedAt, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, shared.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("buscar chargeback: %w", err)
	}
	return chargeback.Restore(cid, txID, mid, reason, shared.Money(amount), chargeback.Status(status), openedAt, deadline, nullTime(resolvedAt), version), nil
}
