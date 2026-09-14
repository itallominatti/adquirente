package chargeback

import (
	"context"

	"github.com/itallominatti/adquirente/internal/application"
	"github.com/itallominatti/adquirente/internal/domain/chargeback"
	"github.com/itallominatti/adquirente/internal/domain/ledger"
	"github.com/itallominatti/adquirente/internal/domain/receivable"
	"github.com/itallominatti/adquirente/internal/domain/shared"
)

type OpenCommand struct {
	MerchantID    string
	TransactionID string
	ReasonCode    string
	Amount        int64
}

type Open struct {
	uow   application.UnitOfWork
	clock shared.Clock
}

func NewOpen(uow application.UnitOfWork, clock shared.Clock) *Open {
	return &Open{uow: uow, clock: clock}
}

func (uc *Open) Execute(ctx context.Context, cmd OpenCommand) (*chargeback.Chargeback, error) {
	var cb *chargeback.Chargeback
	err := uc.uow.Do(ctx, func(repos application.Repositories) error {
		tx, err := repos.Transactions.FindByIDForUpdate(ctx, cmd.MerchantID, cmd.TransactionID)
		if err != nil {
			return err
		}
		now := uc.clock.Now()
		cb, err = chargeback.Open(tx, cmd.ReasonCode, shared.Money(cmd.Amount), now)
		if err != nil {
			return err
		}
		if err := repos.Transactions.Update(ctx, tx); err != nil {
			return err
		}
		if err := repos.Chargebacks.Create(ctx, cb); err != nil {
			return err
		}
		list, err := repos.Receivables.ListByTransaction(ctx, tx.ID())
		if err != nil {
			return err
		}
		var changed []*receivable.Receivable
		var entries []ledger.Entry
		for _, r := range list {
			switch r.Status() {
			case receivable.Scheduled:
				entries = append(entries, ledger.ForChargeback(r, now)...)
			case receivable.Anticipated:
				entries = append(entries, ledger.ForChargebackAnticipated(r, now)...)
			default:
				continue // já liquidado/cancelado: fica como está
			}
			if err := r.Chargeback(now); err != nil {
				return err
			}
			changed = append(changed, r)
		}
		if len(changed) > 0 {
			if err := repos.Receivables.UpdateAll(ctx, changed); err != nil {
				return err
			}
			if err := repos.Ledger.Append(ctx, entries); err != nil {
				return err
			}
		}
		events := append(tx.PullEvents(), cb.PullEvents()...)
		return repos.Outbox.Append(ctx, events...)
	})
	if err != nil {
		return nil, err
	}
	return cb, nil
}
