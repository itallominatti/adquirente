package transaction

import (
	"context"

	"github.com/itallominatti/adquirente/internal/application"
	"github.com/itallominatti/adquirente/internal/domain/shared"
	"github.com/itallominatti/adquirente/internal/domain/transaction"
)

type Lifecycle struct {
	uow   application.UnitOfWork
	clock shared.Clock
}

func NewLifecycle(uow application.UnitOfWork, clock shared.Clock) *Lifecycle {
	return &Lifecycle{uow: uow, clock: clock}
}

func (uc *Lifecycle) Capture(ctx context.Context, merchantID, id string) (*transaction.Transaction, error) {
	return uc.change(ctx, merchantID, id, func(tx *transaction.Transaction) error { return tx.Capture(uc.clock.Now()) })
}

func (uc *Lifecycle) Cancel(ctx context.Context, merchantID, id string) (*transaction.Transaction, error) {
	return uc.change(ctx, merchantID, id, func(tx *transaction.Transaction) error { return tx.Cancel(uc.clock.Now()) })
}

func (uc *Lifecycle) change(ctx context.Context, merchantID, id string, apply func(*transaction.Transaction) error) (*transaction.Transaction, error) {
	var tx *transaction.Transaction
	err := uc.uow.Do(ctx, func(repos application.Repositories) error {
		var err error
		// FOR UPDATE: duas capturas simultâneas da mesma transação ficam em fila;
		// a segunda lê CAPTURED e recebe ErrInvalidTransition (409).
		tx, err = repos.Transactions.FindByIDForUpdate(ctx, merchantID, id)
		if err != nil {
			return err
		}
		if err := apply(tx); err != nil {
			return err
		}
		if err := repos.Transactions.Update(ctx, tx); err != nil {
			return err
		}
		return repos.Outbox.Append(ctx, tx.PullEvents()...)
	})
	if err != nil {
		return nil, err
	}
	return tx, nil
}

type Get struct{ transactions transaction.Repository }

func NewGet(transactions transaction.Repository) *Get { return &Get{transactions: transactions} }

func (uc *Get) Execute(ctx context.Context, merchantID, id string) (*transaction.Transaction, error) {
	return uc.transactions.FindByID(ctx, merchantID, id)
}
