package postgres

import (
	"context"
	"fmt"

	"github.com/itallominatti/adquirente/internal/domain/ledger"
	"github.com/itallominatti/adquirente/internal/domain/shared"
)

type LedgerRepository struct{ db DBTX }

func NewLedgerRepository(db DBTX) *LedgerRepository { return &LedgerRepository{db: db} }

var _ ledger.Repository = (*LedgerRepository)(nil)

func (r *LedgerRepository) Append(ctx context.Context, entries []ledger.Entry) error {
	if !ledger.Balanced(entries) {
		return fmt.Errorf("lote de lançamentos desbalanceado")
	}
	for _, e := range entries {
		_, err := r.db.ExecContext(ctx, `INSERT INTO ledger_entries (id, account, merchant_id, debit, credit, reference_type, reference_id, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			e.ID, string(e.Account), e.MerchantID, int64(e.Debit), int64(e.Credit), e.RefType, e.RefID, e.At)
		if err != nil {
			return fmt.Errorf("inserir lançamento: %w", err)
		}
	}
	return nil
}

func (r *LedgerRepository) Balance(ctx context.Context, account ledger.Account, merchantID string) (shared.Money, error) {
	var v int64
	err := r.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(credit) - SUM(debit), 0) FROM ledger_entries
		WHERE account = $1 AND ($2 = '' OR merchant_id = $2)`, string(account), merchantID).Scan(&v)
	if err != nil {
		return 0, fmt.Errorf("saldo contábil: %w", err)
	}
	return shared.Money(v), nil
}
