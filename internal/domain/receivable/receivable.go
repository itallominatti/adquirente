package receivable

import (
	"fmt"
	"time"

	"github.com/itallominatti/adquirente/internal/domain/shared"
)

type Status string

const (
	Scheduled    Status = "SCHEDULED"    // na agenda, aguardando o vencimento
	Anticipated  Status = "ANTICIPATED"  // o EC já recebeu (com desconto); no vencimento o dinheiro fica com a adquirente
	Settled      Status = "SETTLED"      // pago
	Canceled     Status = "CANCELED"     // a transação foi cancelada antes de liquidar
	Chargebacked Status = "CHARGEBACKED" // contestado; não será pago (ou será estornado)
)

var transitions = map[Status][]Status{
	Scheduled:   {Anticipated, Settled, Canceled, Chargebacked},
	Anticipated: {Settled, Chargebacked},
}

type Receivable struct {
	id             string
	transactionID  string
	merchantID     string
	installmentNo  int
	installments   int
	product        shared.Product
	brand          string
	gross          shared.Money // valor bruto da parcela
	fee            shared.Money // MDR da parcela
	net            shared.Money // o que o EC recebe (gross - fee)
	dueDate        time.Time    // data de liquidação (dia útil)
	status         Status
	settlementID   string
	anticipationID string
	version        int
	createdAt      time.Time
	updatedAt      time.Time
}

func newReceivable(transactionID, merchantID string, installmentNo, installments int, product shared.Product, brand string,
	gross, fee shared.Money, dueDate, now time.Time) *Receivable {
	return &Receivable{
		id: shared.NewID("rcv"), transactionID: transactionID, merchantID: merchantID,
		installmentNo: installmentNo, installments: installments, product: product, brand: brand,
		gross: gross, fee: fee, net: gross - fee, dueDate: shared.DateOnly(dueDate),
		status: Scheduled, version: 1, createdAt: now, updatedAt: now,
	}
}

func Restore(id, transactionID, merchantID string, installmentNo, installments int, product shared.Product, brand string,
	gross, fee, net shared.Money, dueDate time.Time, status Status, settlementID, anticipationID string,
	version int, createdAt, updatedAt time.Time) *Receivable {
	return &Receivable{
		id: id, transactionID: transactionID, merchantID: merchantID, installmentNo: installmentNo, installments: installments,
		product: product, brand: brand, gross: gross, fee: fee, net: net, dueDate: dueDate, status: status,
		settlementID: settlementID, anticipationID: anticipationID, version: version, createdAt: createdAt, updatedAt: updatedAt,
	}
}

func (r *Receivable) transition(to Status, now time.Time) error {
	for _, allowed := range transitions[r.status] {
		if allowed == to {
			r.status = to
			r.version++
			r.updatedAt = now
			return nil
		}
	}
	return fmt.Errorf("%w: recebível %s %s → %s", shared.ErrInvalidTransition, r.id, r.status, to)
}

func (r *Receivable) IsAnticipable(today time.Time) bool {
	return r.status == Scheduled && r.dueDate.After(shared.DateOnly(today))
}

func (r *Receivable) DaysUntilDue(today time.Time) int {
	return int(r.dueDate.Sub(shared.DateOnly(today)).Hours() / 24)
}

func (r *Receivable) Anticipate(anticipationID string, now time.Time) error {
	if err := r.transition(Anticipated, now); err != nil {
		return err
	}
	r.anticipationID = anticipationID
	return nil
}

func (r *Receivable) Settle(settlementID string, now time.Time) error {
	if err := r.transition(Settled, now); err != nil {
		return err
	}
	r.settlementID = settlementID
	return nil
}

func (r *Receivable) Cancel(now time.Time) error     { return r.transition(Canceled, now) }
func (r *Receivable) Chargeback(now time.Time) error { return r.transition(Chargebacked, now) }

func (r *Receivable) ID() string              { return r.id }
func (r *Receivable) TransactionID() string   { return r.transactionID }
func (r *Receivable) MerchantID() string      { return r.merchantID }
func (r *Receivable) InstallmentNo() int      { return r.installmentNo }
func (r *Receivable) Installments() int       { return r.installments }
func (r *Receivable) Product() shared.Product { return r.product }
func (r *Receivable) Brand() string           { return r.brand }
func (r *Receivable) Gross() shared.Money     { return r.gross }
func (r *Receivable) Fee() shared.Money       { return r.fee }
func (r *Receivable) Net() shared.Money       { return r.net }
func (r *Receivable) DueDate() time.Time      { return r.dueDate }
func (r *Receivable) Status() Status          { return r.status }
func (r *Receivable) SettlementID() string    { return r.settlementID }
func (r *Receivable) AnticipationID() string  { return r.anticipationID }
func (r *Receivable) Version() int            { return r.version }
func (r *Receivable) CreatedAt() time.Time    { return r.createdAt }
func (r *Receivable) UpdatedAt() time.Time    { return r.updatedAt }
