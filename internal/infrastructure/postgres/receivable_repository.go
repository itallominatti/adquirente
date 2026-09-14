package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/itallominatti/adquirente/internal/domain/receivable"
	"github.com/itallominatti/adquirente/internal/domain/shared"
)

type ReceivableRepository struct{ db DBTX }

func NewReceivableRepository(db DBTX) *ReceivableRepository { return &ReceivableRepository{db: db} }

var _ receivable.Repository = (*ReceivableRepository)(nil)

const rcvColumns = `id, transaction_id, merchant_id, installment_no, installments, product, brand, gross_amount, fee_amount,
	net_amount, due_date, status, settlement_id, anticipation_id, version, created_at, updated_at`

func (r *ReceivableRepository) SaveAll(ctx context.Context, list []*receivable.Receivable) error {
	for _, x := range list {
		_, err := r.db.ExecContext(ctx, `INSERT INTO receivables (`+rcvColumns+`)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
			x.ID(), x.TransactionID(), x.MerchantID(), x.InstallmentNo(), x.Installments(), string(x.Product()), x.Brand(),
			int64(x.Gross()), int64(x.Fee()), int64(x.Net()), x.DueDate(), string(x.Status()), x.SettlementID(), x.AnticipationID(),
			x.Version(), x.CreatedAt(), x.UpdatedAt())
		if err != nil {
			return fmt.Errorf("inserir recebível %s: %w", x.ID(), err)
		}
	}
	return nil
}

func (r *ReceivableRepository) ListByMerchant(ctx context.Context, merchantID string, f receivable.ListFilter) ([]*receivable.Receivable, error) {
	// SQL montado com PARÂMETROS ($n), nunca com concatenação de valores: sem SQL injection.
	var where []string
	args := []any{merchantID}
	where = append(where, "merchant_id = $1")
	if f.Status != "" {
		args = append(args, string(f.Status))
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	if !f.From.IsZero() {
		args = append(args, shared.DateOnly(f.From))
		where = append(where, fmt.Sprintf("due_date >= $%d", len(args)))
	}
	if !f.To.IsZero() {
		args = append(args, shared.DateOnly(f.To))
		where = append(where, fmt.Sprintf("due_date <= $%d", len(args)))
	}
	args = append(args, f.Limit)
	query := `SELECT ` + rcvColumns + ` FROM receivables WHERE ` + strings.Join(where, " AND ") +
		fmt.Sprintf(" ORDER BY due_date, installment_no LIMIT $%d", len(args))
	return r.list(ctx, query, args...)
}

func (r *ReceivableRepository) ListByTransaction(ctx context.Context, transactionID string) ([]*receivable.Receivable, error) {
	return r.list(ctx, `SELECT `+rcvColumns+` FROM receivables WHERE transaction_id = $1 ORDER BY installment_no FOR UPDATE`, transactionID)
}

func (r *ReceivableRepository) ListDueOnForUpdate(ctx context.Context, day time.Time, limit int) ([]*receivable.Receivable, error) {
	return r.list(ctx, `SELECT `+rcvColumns+` FROM receivables
		WHERE due_date = $1 AND status IN ('SCHEDULED', 'ANTICIPATED')
		ORDER BY merchant_id, id LIMIT $2 FOR UPDATE SKIP LOCKED`, shared.DateOnly(day), limit)
}

func (r *ReceivableRepository) FindByIDs(ctx context.Context, merchantID string, ids []string) ([]*receivable.Receivable, error) {
	return r.list(ctx, `SELECT `+rcvColumns+` FROM receivables WHERE merchant_id = $1 AND id = ANY($2) ORDER BY due_date, id`, merchantID, ids)
}

func (r *ReceivableRepository) FindForUpdate(ctx context.Context, merchantID string, ids []string) ([]*receivable.Receivable, error) {
	// ORDER BY id: travar sempre na mesma ordem evita deadlock entre dois pedidos concorrentes.
	return r.list(ctx, `SELECT `+rcvColumns+` FROM receivables WHERE merchant_id = $1 AND id = ANY($2) ORDER BY id FOR UPDATE`, merchantID, ids)
}

func (r *ReceivableRepository) UpdateAll(ctx context.Context, list []*receivable.Receivable) error {
	for _, x := range list {
		res, err := r.db.ExecContext(ctx, `UPDATE receivables SET status = $2, settlement_id = $3, anticipation_id = $4, version = $5, updated_at = $6
			WHERE id = $1 AND version < $5`,
			x.ID(), string(x.Status()), x.SettlementID(), x.AnticipationID(), x.Version(), x.UpdatedAt())
		if err != nil {
			return fmt.Errorf("atualizar recebível %s: %w", x.ID(), err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return shared.ErrConcurrentUpdate
		}
	}
	return nil
}

func (r *ReceivableRepository) Summary(ctx context.Context, merchantID string, from, to time.Time) ([]receivable.DailySummary, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT due_date, status, COUNT(*), COALESCE(SUM(net_amount), 0)
		FROM receivables WHERE merchant_id = $1 AND due_date BETWEEN $2 AND $3
		GROUP BY due_date, status ORDER BY due_date, status`, merchantID, shared.DateOnly(from), shared.DateOnly(to))
	if err != nil {
		return nil, fmt.Errorf("resumo da agenda: %w", err)
	}
	defer rows.Close()
	var out []receivable.DailySummary
	for rows.Next() {
		var s receivable.DailySummary
		var status string
		var net int64
		if err := rows.Scan(&s.DueDate, &status, &s.Count, &net); err != nil {
			return nil, err
		}
		s.Status, s.Net = receivable.Status(status), shared.Money(net)
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *ReceivableRepository) list(ctx context.Context, query string, args ...any) ([]*receivable.Receivable, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listar recebíveis: %w", err)
	}
	defer rows.Close()
	var out []*receivable.Receivable
	for rows.Next() {
		x, err := scanReceivable(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func scanReceivable(rows *sql.Rows) (*receivable.Receivable, error) {
	var (
		id, txID, merchantID, product, brand, status, settlementID, anticipationID string
		installmentNo, installments, version                                       int
		gross, fee, net                                                            int64
		dueDate, createdAt, updatedAt                                              time.Time
	)
	if err := rows.Scan(&id, &txID, &merchantID, &installmentNo, &installments, &product, &brand, &gross, &fee, &net,
		&dueDate, &status, &settlementID, &anticipationID, &version, &createdAt, &updatedAt); err != nil {
		return nil, fmt.Errorf("ler recebível: %w", err)
	}
	return receivable.Restore(id, txID, merchantID, installmentNo, installments, shared.Product(product), brand,
		shared.Money(gross), shared.Money(fee), shared.Money(net), shared.DateOnly(dueDate), receivable.Status(status),
		settlementID, anticipationID, version, createdAt, updatedAt), nil
}
