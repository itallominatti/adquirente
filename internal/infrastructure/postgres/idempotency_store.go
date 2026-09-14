package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/itallominatti/adquirente/internal/application"
)

type IdempotencyStore struct {
	db  *sql.DB
	ttl time.Duration
}

var _ application.IdempotencyStore = (*IdempotencyStore)(nil)

func NewIdempotencyStore(db *sql.DB, ttl time.Duration) *IdempotencyStore {
	return &IdempotencyStore{db: db, ttl: ttl}
}

func (s *IdempotencyStore) Begin(ctx context.Context, merchantID, key, requestHash string) (application.IdempotencyResult, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO idempotency_keys (merchant_id, idempotency_key, request_hash, expires_at)
		VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`, merchantID, key, requestHash, time.Now().Add(s.ttl))
	if err != nil {
		return application.IdempotencyResult{}, fmt.Errorf("reservar chave de idempotência: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 1 {
		return application.IdempotencyResult{State: application.IdempotencyNew}, nil
	}

	var storedHash string
	var status sql.NullInt64
	var body []byte
	err = s.db.QueryRowContext(ctx, `SELECT request_hash, response_status, response_body FROM idempotency_keys
		WHERE merchant_id = $1 AND idempotency_key = $2`, merchantID, key).Scan(&storedHash, &status, &body)
	if errors.Is(err, sql.ErrNoRows) { // a chave foi abandonada entre o INSERT e o SELECT: peça para tentar de novo
		return application.IdempotencyResult{State: application.IdempotencyInProgress}, nil
	}
	if err != nil {
		return application.IdempotencyResult{}, fmt.Errorf("ler chave de idempotência: %w", err)
	}
	switch {
	case storedHash != requestHash:
		return application.IdempotencyResult{State: application.IdempotencyConflict}, nil
	case !status.Valid:
		return application.IdempotencyResult{State: application.IdempotencyInProgress}, nil
	}
	return application.IdempotencyResult{State: application.IdempotencyReplay, Status: int(status.Int64), Body: body}, nil
}

func (s *IdempotencyStore) Complete(ctx context.Context, merchantID, key string, status int, body []byte) error {
	_, err := s.db.ExecContext(ctx, `UPDATE idempotency_keys SET response_status = $3, response_body = $4
		WHERE merchant_id = $1 AND idempotency_key = $2`, merchantID, key, status, body)
	return err
}

func (s *IdempotencyStore) Abandon(ctx context.Context, merchantID, key string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM idempotency_keys WHERE merchant_id = $1 AND idempotency_key = $2 AND response_status IS NULL`, merchantID, key)
	return err
}
