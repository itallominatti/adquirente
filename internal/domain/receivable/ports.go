package receivable

import (
	"context"
	"time"

	"github.com/itallominatti/adquirente/internal/domain/merchant"
	"github.com/itallominatti/adquirente/internal/domain/shared"
)

type ListFilter struct {
	Status Status    // vazio = todos
	From   time.Time // vencimento >= From (zero = sem limite)
	To     time.Time // vencimento <= To (zero = sem limite)
	Limit  int
}

type DailySummary struct {
	DueDate time.Time
	Status  Status
	Count   int
	Net     shared.Money
}

type Repository interface {
	SaveAll(ctx context.Context, list []*Receivable) error
	ListByMerchant(ctx context.Context, merchantID string, f ListFilter) ([]*Receivable, error)
	ListByTransaction(ctx context.Context, transactionID string) ([]*Receivable, error)
	// ListDueOnForUpdate trava as linhas que vencem no dia (FOR UPDATE SKIP LOCKED):
	// vários workers podem rodar em paralelo sem pegar o mesmo recebível.
	ListDueOnForUpdate(ctx context.Context, day time.Time, limit int) ([]*Receivable, error)
	// FindByIDs lê recebíveis específicos de um EC sem travar (simulação de antecipação).
	FindByIDs(ctx context.Context, merchantID string, ids []string) ([]*Receivable, error)
	// FindForUpdate trava recebíveis específicos de um EC (antecipação).
	FindForUpdate(ctx context.Context, merchantID string, ids []string) ([]*Receivable, error)
	UpdateAll(ctx context.Context, list []*Receivable) error
	Summary(ctx context.Context, merchantID string, from, to time.Time) ([]DailySummary, error)
}

type Lien struct {
	MerchantID   string
	DueDate      time.Time
	Amount       shared.Money
	CreditorName string
	Creditor     merchant.BankAccount
}

type Registry interface {
	Register(ctx context.Context, units []Unit) error
	CheckLiens(ctx context.Context, merchantID string, dueDate time.Time) ([]Lien, error)
	// TransferOwnership informa que a adquirente passou a ser dona (antecipação).
	TransferOwnership(ctx context.Context, receivableIDs []string, newOwner string) error
}

// Eventos publicados pelos casos de uso que mexem na agenda.

type ReceivablesScheduled struct {
	shared.BaseEvent
	TransactionID string       `json:"transaction_id"`
	MerchantID    string       `json:"merchant_id"`
	Count         int          `json:"count"`
	GrossTotal    shared.Money `json:"gross_total"`
	NetTotal      shared.Money `json:"net_total"`
	FirstDueDate  time.Time    `json:"first_due_date"`
	LastDueDate   time.Time    `json:"last_due_date"`
}

func NewReceivablesScheduled(list []*Receivable, now time.Time) ReceivablesScheduled {
	ev := ReceivablesScheduled{BaseEvent: shared.NewBaseEvent("receivable.scheduled", list[0].TransactionID(), now),
		TransactionID: list[0].TransactionID(), MerchantID: list[0].MerchantID(), Count: len(list)}
	ev.FirstDueDate, ev.LastDueDate = list[0].DueDate(), list[0].DueDate()
	for _, r := range list {
		ev.GrossTotal += r.Gross()
		ev.NetTotal += r.Net()
		if r.DueDate().Before(ev.FirstDueDate) {
			ev.FirstDueDate = r.DueDate()
		}
		if r.DueDate().After(ev.LastDueDate) {
			ev.LastDueDate = r.DueDate()
		}
	}
	return ev
}
