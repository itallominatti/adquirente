package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/itallominatti/adquirente/internal/domain/shared"
)

type CardRecord struct {
	Token             string
	PANCiphertext     []byte
	ExpiryCiphertext  []byte
	Brand, BIN, Last4 string
}

type CardVaultStore struct{ db *sql.DB }

func NewCardVaultStore(db *sql.DB) *CardVaultStore { return &CardVaultStore{db: db} }

func (s *CardVaultStore) Save(ctx context.Context, rec CardRecord) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO card_vault (token, pan_ciphertext, expiry_ciphertext, brand, bin, last4)
		VALUES ($1,$2,$3,$4,$5,$6)`, rec.Token, rec.PANCiphertext, rec.ExpiryCiphertext, rec.Brand, rec.BIN, rec.Last4)
	if err != nil {
		return fmt.Errorf("gravar cartão no cofre: %w", err)
	}
	return nil
}

func (s *CardVaultStore) Find(ctx context.Context, token string) (CardRecord, error) {
	var rec CardRecord
	err := s.db.QueryRowContext(ctx, `SELECT token, pan_ciphertext, expiry_ciphertext, brand, bin, last4 FROM card_vault WHERE token = $1`, token).
		Scan(&rec.Token, &rec.PANCiphertext, &rec.ExpiryCiphertext, &rec.Brand, &rec.BIN, &rec.Last4)
	if errors.Is(err, sql.ErrNoRows) {
		return CardRecord{}, shared.ErrNotFound
	}
	if err != nil {
		return CardRecord{}, fmt.Errorf("ler cartão do cofre: %w", err)
	}
	return rec, nil
}
