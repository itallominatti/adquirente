package chargeback

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/itallominatti/adquirente/internal/domain/shared"
	"github.com/itallominatti/adquirente/internal/domain/transaction"
)

type Status string

const (
	Opened   Status = "OPENED"   // emissor contestou; valor retido/estornado do EC
	Defended Status = "DEFENDED" // EC apresentou documentos
	Accepted Status = "ACCEPTED" // EC perdeu: estorno definitivo
	Reversed Status = "REVERSED" // EC ganhou: valor devolvido ao EC
)

var transitions = map[Status][]Status{
	Opened:   {Defended, Accepted, Reversed},
	Defended: {Accepted, Reversed},
}

var (
	ErrInvalidAmount     = errors.New("valor do chargeback maior que o da transação")
	ErrTransactionStatus = errors.New("só transações capturadas ou liquidadas podem ser contestadas")
)

type Chargeback struct {
	id            string
	transactionID string
	merchantID    string
	reasonCode    string // código da bandeira, ex.: "4837" (fraude, Mastercard), "10.4" (Visa)
	amount        shared.Money
	status        Status
	openedAt      time.Time
	deadline      time.Time // prazo para o EC se defender
	resolvedAt    *time.Time
	version       int
	events        []shared.Event
}

const defenseDays = 10

func Open(tx *transaction.Transaction, reasonCode string, amount shared.Money, now time.Time) (*Chargeback, error) {
	if tx.Status() != transaction.Captured && tx.Status() != transaction.Settled {
		return nil, ErrTransactionStatus
	}
	if !amount.IsPositive() || amount > tx.Amount() {
		return nil, ErrInvalidAmount
	}
	if err := tx.Chargeback(reasonCode, now); err != nil {
		return nil, err
	}
	c := &Chargeback{
		id: shared.NewID("cb"), transactionID: tx.ID(), merchantID: tx.MerchantID(), reasonCode: reasonCode,
		amount: amount, status: Opened, openedAt: now, deadline: now.AddDate(0, 0, defenseDays), version: 1,
	}
	c.events = append(c.events, ChargebackOpened{
		BaseEvent:    shared.NewBaseEvent("chargeback.opened", c.id, now),
		ChargebackID: c.id, TransactionID: tx.ID(), MerchantID: tx.MerchantID(), Amount: amount, ReasonCode: reasonCode, Deadline: c.deadline,
	})
	return c, nil
}

func (c *Chargeback) transition(to Status) error {
	for _, allowed := range transitions[c.status] {
		if allowed == to {
			c.status = to
			c.version++
			return nil
		}
	}
	return fmt.Errorf("%w: %s → %s", shared.ErrInvalidTransition, c.status, to)
}

func (c *Chargeback) Defend(now time.Time) error {
	if now.After(c.deadline) {
		return errors.New("prazo de defesa encerrado")
	}
	return c.transition(Defended)
}

func (c *Chargeback) Accept(now time.Time) error  { return c.resolve(Accepted, now) }
func (c *Chargeback) Reverse(now time.Time) error { return c.resolve(Reversed, now) }

func (c *Chargeback) resolve(to Status, now time.Time) error {
	if err := c.transition(to); err != nil {
		return err
	}
	c.resolvedAt = &now
	c.events = append(c.events, ChargebackResolved{
		BaseEvent:    shared.NewBaseEvent("chargeback.resolved", c.id, now),
		ChargebackID: c.id, TransactionID: c.transactionID, MerchantID: c.merchantID, Amount: c.amount, Outcome: string(to),
	})
	return nil
}

func Restore(id, transactionID, merchantID, reasonCode string, amount shared.Money, status Status, openedAt, deadline time.Time, resolvedAt *time.Time, version int) *Chargeback {
	return &Chargeback{id: id, transactionID: transactionID, merchantID: merchantID, reasonCode: reasonCode, amount: amount,
		status: status, openedAt: openedAt, deadline: deadline, resolvedAt: resolvedAt, version: version}
}

func (c *Chargeback) PullEvents() []shared.Event { evs := c.events; c.events = nil; return evs }

func (c *Chargeback) ID() string             { return c.id }
func (c *Chargeback) TransactionID() string  { return c.transactionID }
func (c *Chargeback) MerchantID() string     { return c.merchantID }
func (c *Chargeback) ReasonCode() string     { return c.reasonCode }
func (c *Chargeback) Amount() shared.Money   { return c.amount }
func (c *Chargeback) Status() Status         { return c.status }
func (c *Chargeback) OpenedAt() time.Time    { return c.openedAt }
func (c *Chargeback) Deadline() time.Time    { return c.deadline }
func (c *Chargeback) ResolvedAt() *time.Time { return c.resolvedAt }
func (c *Chargeback) Version() int           { return c.version }

type Repository interface {
	Create(ctx context.Context, c *Chargeback) error
	Update(ctx context.Context, c *Chargeback) error
	FindByID(ctx context.Context, merchantID, id string) (*Chargeback, error)
}

type ChargebackOpened struct {
	shared.BaseEvent
	ChargebackID  string       `json:"chargeback_id"`
	TransactionID string       `json:"transaction_id"`
	MerchantID    string       `json:"merchant_id"`
	Amount        shared.Money `json:"amount"`
	ReasonCode    string       `json:"reason_code"`
	Deadline      time.Time    `json:"deadline"`
}

type ChargebackResolved struct {
	shared.BaseEvent
	ChargebackID  string       `json:"chargeback_id"`
	TransactionID string       `json:"transaction_id"`
	MerchantID    string       `json:"merchant_id"`
	Amount        shared.Money `json:"amount"`
	Outcome       string       `json:"outcome"`
}
