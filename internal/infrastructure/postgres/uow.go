package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/itallominatti/adquirente/internal/application"
)

type UnitOfWork struct{ db *sql.DB }

func NewUnitOfWork(db *sql.DB) *UnitOfWork { return &UnitOfWork{db: db} }

func (u *UnitOfWork) Do(ctx context.Context, fn func(repos application.Repositories) error) error {
	tx, err := u.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("iniciar transação: %w", err)
	}
	// Rollback depois de Commit é inofensivo (devolve ErrTxDone). Garante o "nada" do "tudo ou nada".
	defer func() { _ = tx.Rollback() }()

	if err := fn(NewRepositories(tx)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("confirmar transação: %w", err)
	}
	return nil
}

func NewRepositories(db DBTX) application.Repositories {
	return application.Repositories{
		Transactions:  NewTransactionRepository(db),
		Receivables:   NewReceivableRepository(db),
		Settlements:   NewSettlementRepository(db),
		Anticipations: NewAnticipationRepository(db),
		Chargebacks:   NewChargebackRepository(db),
		Ledger:        NewLedgerRepository(db),
		Outbox:        NewOutboxRepository(db),
		Processed:     NewProcessedEventsRepository(db),
		NSU:           NewNSUGenerator(db),
	}
}
