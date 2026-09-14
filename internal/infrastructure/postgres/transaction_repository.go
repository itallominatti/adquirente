package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/itallominatti/adquirente/internal/domain/shared"
	"github.com/itallominatti/adquirente/internal/domain/transaction"
)

type TransactionRepository struct{ db DBTX }

func NewTransactionRepository(db DBTX) *TransactionRepository { return &TransactionRepository{db: db} }

var _ transaction.Repository = (*TransactionRepository)(nil)

const txColumns = `id, merchant_id, amount, product, installments, status, card_token, card_brand, card_bin, card_last4,
	authorization_code, nsu, response_code, created_at, authorized_at, captured_at, canceled_at, version`

func (r *TransactionRepository) Create(ctx context.Context, t *transaction.Transaction) error {
	c := t.Card()
	_, err := r.db.ExecContext(ctx, `INSERT INTO transactions (`+txColumns+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`,
		t.ID(), t.MerchantID(), int64(t.Amount()), string(t.Product()), t.Installments(), string(t.Status()),
		c.Token, c.Brand, c.BIN, c.Last4, t.AuthorizationCode(), t.NSU(), t.ResponseCode(),
		t.CreatedAt(), t.AuthorizedAt(), t.CapturedAt(), t.CanceledAt(), t.Version())
	if err != nil {
		return fmt.Errorf("inserir transação: %w", err)
	}
	return nil
}

func (r *TransactionRepository) FindByID(ctx context.Context, merchantID, id string) (*transaction.Transaction, error) {
	return r.findOne(ctx, `SELECT `+txColumns+` FROM transactions WHERE id = $1 AND merchant_id = $2`, id, merchantID)
}

func (r *TransactionRepository) FindByIDForUpdate(ctx context.Context, merchantID, id string) (*transaction.Transaction, error) {
	return r.findOne(ctx, `SELECT `+txColumns+` FROM transactions WHERE id = $1 AND merchant_id = $2 FOR UPDATE`, id, merchantID)
}

func (r *TransactionRepository) Update(ctx context.Context, t *transaction.Transaction) error {
	res, err := r.db.ExecContext(ctx, `UPDATE transactions SET status = $2, authorization_code = $3, nsu = $4, response_code = $5,
		authorized_at = $6, captured_at = $7, canceled_at = $8, version = $9
		WHERE id = $1 AND version < $9`,
		t.ID(), string(t.Status()), t.AuthorizationCode(), t.NSU(), t.ResponseCode(),
		t.AuthorizedAt(), t.CapturedAt(), t.CanceledAt(), t.Version())
	if err != nil {
		return fmt.Errorf("atualizar transação: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return shared.ErrConcurrentUpdate
	}
	return nil
}

func (r *TransactionRepository) findOne(ctx context.Context, query string, args ...any) (*transaction.Transaction, error) {
	var (
		id, merchantID, product, status, token, brand, bin, last4, authCode, nsu, respCode string
		amount                                                                             int64
		installments, version                                                              int
		createdAt                                                                          time.Time
		authorizedAt, capturedAt, canceledAt                                               sql.NullTime
	)
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&id, &merchantID, &amount, &product, &installments, &status,
		&token, &brand, &bin, &last4, &authCode, &nsu, &respCode, &createdAt, &authorizedAt, &capturedAt, &canceledAt, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, shared.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("buscar transação: %w", err)
	}
	return transaction.Restore(id, merchantID, shared.Money(amount), shared.Product(product), installments,
		transaction.CardInfo{Token: token, Brand: brand, BIN: bin, Last4: last4},
		transaction.Status(status), authCode, nsu, respCode, createdAt,
		nullTime(authorizedAt), nullTime(capturedAt), nullTime(canceledAt), version), nil
}

func nullTime(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

type NSUGenerator struct{ db DBTX }

func NewNSUGenerator(db DBTX) *NSUGenerator { return &NSUGenerator{db: db} }

func (g *NSUGenerator) Next(ctx context.Context) (string, error) {
	var n int64
	if err := g.db.QueryRowContext(ctx, `SELECT nextval('nsu_seq')`).Scan(&n); err != nil {
		return "", fmt.Errorf("gerar NSU: %w", err)
	}
	return fmt.Sprintf("%012d", n), nil
}
