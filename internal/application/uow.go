package application

import (
	"context"

	"github.com/itallominatti/adquirente/internal/domain/anticipation"
	"github.com/itallominatti/adquirente/internal/domain/chargeback"
	"github.com/itallominatti/adquirente/internal/domain/ledger"
	"github.com/itallominatti/adquirente/internal/domain/receivable"
	"github.com/itallominatti/adquirente/internal/domain/settlement"
	"github.com/itallominatti/adquirente/internal/domain/shared"
	"github.com/itallominatti/adquirente/internal/domain/transaction"
)

type Repositories struct {
	Transactions  transaction.Repository
	Receivables   receivable.Repository
	Settlements   settlement.Repository
	Anticipations anticipation.Repository
	Chargebacks   chargeback.Repository
	Ledger        ledger.Repository
	Outbox        OutboxRepository
	Processed     ProcessedEventsRepository
	NSU           transaction.NSUGenerator
}

type UnitOfWork interface {
	Do(ctx context.Context, fn func(repos Repositories) error) error
}

type OutboxRepository interface {
	Append(ctx context.Context, events ...shared.Event) error
}

type ProcessedEventsRepository interface {
	MarkIfNew(ctx context.Context, consumer, eventID string) (bool, error)
}
