package settlement

import (
	"errors"
	"fmt"
	"time"

	"github.com/itallominatti/adquirente/internal/domain/merchant"
	"github.com/itallominatti/adquirente/internal/domain/receivable"
	"github.com/itallominatti/adquirente/internal/domain/shared"
)

type Status string

const (
	Pending   Status = "PENDING"   // montada, ainda não enviada ao banco
	Sent      Status = "SENT"      // ordens enviadas; aguardando retorno
	Confirmed Status = "CONFIRMED" // banco confirmou o crédito
	Failed    Status = "FAILED"    // banco rejeitou (conta inválida etc.)
)

type OrderKind string

const (
	ToMerchant OrderKind = "TO_MERCHANT" // o normal
	ToCreditor OrderKind = "TO_CREDITOR" // havia ônus registrado: paga o credor
)

type Order struct {
	ID          string               `json:"id"`
	Kind        OrderKind            `json:"kind"`
	Beneficiary string               `json:"beneficiary"`
	BankAccount merchant.BankAccount `json:"bank_account"`
	Amount      shared.Money         `json:"amount"`
}

var (
	ErrNothingToSettle = errors.New("nenhum recebível para liquidar")
	ErrWrongDay        = errors.New("recebível não vence na data da liquidação")
	ErrWrongMerchant   = errors.New("recebível de outro estabelecimento no lote")
	ErrWrongStatus     = errors.New("recebível em estado que não permite liquidação")
)

type Settlement struct {
	id            string
	date          time.Time
	merchantID    string
	total         shared.Money // soma dos recebíveis do lote (líquido)
	toMerchant    shared.Money // o que vai para a conta do EC
	toCreditors   shared.Money // o que vai para credores (ônus)
	retained      shared.Money // recebíveis antecipados: já pagos; o dinheiro fica com a adquirente
	orders        []Order
	receivableIDs []string
	status        Status
	externalRef   string
	failureReason string
	createdAt     time.Time
	updatedAt     time.Time
	version       int
	events        []shared.Event
}

func Build(date time.Time, m *merchant.Merchant, receivables []*receivable.Receivable, liens []receivable.Lien, now time.Time) (*Settlement, error) {
	if len(receivables) == 0 {
		return nil, ErrNothingToSettle
	}
	day := shared.DateOnly(date)
	s := &Settlement{id: shared.NewID("stl"), date: day, merchantID: m.ID(), status: Pending, createdAt: now, updatedAt: now, version: 1}

	var payable shared.Money
	for _, r := range receivables {
		switch {
		case r.MerchantID() != m.ID():
			return nil, fmt.Errorf("%w: %s", ErrWrongMerchant, r.ID())
		case !r.DueDate().Equal(day):
			return nil, fmt.Errorf("%w: %s vence %s", ErrWrongDay, r.ID(), r.DueDate().Format("2006-01-02"))
		}
		switch r.Status() {
		case receivable.Scheduled:
			payable += r.Net()
		case receivable.Anticipated:
			s.retained += r.Net()
		default:
			return nil, fmt.Errorf("%w: %s está %s", ErrWrongStatus, r.ID(), r.Status())
		}
		s.total += r.Net()
		s.receivableIDs = append(s.receivableIDs, r.ID())
	}

	// Ônus primeiro: o credor tem prioridade sobre o EC, até o limite do que há para pagar.
	for _, l := range liens {
		if payable == 0 {
			break
		}
		amount := l.Amount
		if amount > payable {
			amount = payable
		}
		s.orders = append(s.orders, Order{ID: shared.NewID("ord"), Kind: ToCreditor, Beneficiary: l.CreditorName, BankAccount: l.Creditor, Amount: amount})
		s.toCreditors += amount
		payable -= amount
	}
	if payable > 0 {
		s.orders = append(s.orders, Order{ID: shared.NewID("ord"), Kind: ToMerchant, Beneficiary: m.LegalName(), BankAccount: m.BankAccount(), Amount: payable})
		s.toMerchant = payable
	}
	return s, nil
}

func (s *Settlement) MarkSent(externalRef string, now time.Time) error {
	if s.status != Pending {
		return fmt.Errorf("%w: %s → %s", shared.ErrInvalidTransition, s.status, Sent)
	}
	s.status, s.externalRef, s.updatedAt = Sent, externalRef, now
	s.version++
	return nil
}

func (s *Settlement) MarkConfirmed(now time.Time) error {
	if s.status != Sent {
		return fmt.Errorf("%w: %s → %s", shared.ErrInvalidTransition, s.status, Confirmed)
	}
	s.status, s.updatedAt = Confirmed, now
	s.version++
	s.events = append(s.events, SettlementCompleted{
		BaseEvent:    shared.NewBaseEvent("settlement.completed", s.id, now),
		SettlementID: s.id, MerchantID: s.merchantID, Date: s.date, Total: s.total, ToMerchant: s.toMerchant,
		ToCreditors: s.toCreditors, Retained: s.retained, ReceivableIDs: s.receivableIDs,
	})
	return nil
}

func (s *Settlement) MarkFailed(reason string, now time.Time) error {
	if s.status != Sent && s.status != Pending {
		return fmt.Errorf("%w: %s → %s", shared.ErrInvalidTransition, s.status, Failed)
	}
	s.status, s.failureReason, s.updatedAt = Failed, reason, now
	s.version++
	s.events = append(s.events, SettlementFailed{
		BaseEvent:    shared.NewBaseEvent("settlement.failed", s.id, now),
		SettlementID: s.id, MerchantID: s.merchantID, Date: s.date, Reason: reason, ReceivableIDs: s.receivableIDs,
	})
	return nil
}

func Restore(id string, date time.Time, merchantID string, total, toMerchant, toCreditors, retained shared.Money, orders []Order,
	receivableIDs []string, status Status, externalRef, failureReason string, createdAt, updatedAt time.Time, version int) *Settlement {
	return &Settlement{id: id, date: date, merchantID: merchantID, total: total, toMerchant: toMerchant, toCreditors: toCreditors,
		retained: retained, orders: orders, receivableIDs: receivableIDs, status: status, externalRef: externalRef,
		failureReason: failureReason, createdAt: createdAt, updatedAt: updatedAt, version: version}
}

func (s *Settlement) PullEvents() []shared.Event { evs := s.events; s.events = nil; return evs }

func (s *Settlement) ID() string                { return s.id }
func (s *Settlement) Date() time.Time           { return s.date }
func (s *Settlement) MerchantID() string        { return s.merchantID }
func (s *Settlement) Total() shared.Money       { return s.total }
func (s *Settlement) ToMerchant() shared.Money  { return s.toMerchant }
func (s *Settlement) ToCreditors() shared.Money { return s.toCreditors }
func (s *Settlement) Retained() shared.Money    { return s.retained }
func (s *Settlement) Orders() []Order           { return s.orders }
func (s *Settlement) ReceivableIDs() []string   { return s.receivableIDs }
func (s *Settlement) Status() Status            { return s.status }
func (s *Settlement) ExternalRef() string       { return s.externalRef }
func (s *Settlement) FailureReason() string     { return s.failureReason }
func (s *Settlement) CreatedAt() time.Time      { return s.createdAt }
func (s *Settlement) UpdatedAt() time.Time      { return s.updatedAt }
func (s *Settlement) Version() int              { return s.version }

type SettlementCompleted struct {
	shared.BaseEvent
	SettlementID  string       `json:"settlement_id"`
	MerchantID    string       `json:"merchant_id"`
	Date          time.Time    `json:"date"`
	Total         shared.Money `json:"total"`
	ToMerchant    shared.Money `json:"to_merchant"`
	ToCreditors   shared.Money `json:"to_creditors"`
	Retained      shared.Money `json:"retained"`
	ReceivableIDs []string     `json:"receivable_ids"`
}

type SettlementFailed struct {
	shared.BaseEvent
	SettlementID  string    `json:"settlement_id"`
	MerchantID    string    `json:"merchant_id"`
	Date          time.Time `json:"date"`
	Reason        string    `json:"reason"`
	ReceivableIDs []string  `json:"receivable_ids"`
}
