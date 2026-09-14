package transaction

import (
	"errors"
	"fmt"
	"time"

	"github.com/itallominatti/adquirente/internal/domain/merchant"
	"github.com/itallominatti/adquirente/internal/domain/shared"
)

// Status é o estado da transação de cartão. Veja a tabela transitions abaixo:
// toda mudança passa por lá, então um estado inválido é impossível por construção.
type Status string

const (
	Pending      Status = "PENDING"
	Authorized   Status = "AUTHORIZED"
	Denied       Status = "DENIED"
	Captured     Status = "CAPTURED"
	Canceled     Status = "CANCELED"
	Settled      Status = "SETTLED"
	Chargebacked Status = "CHARGEBACKED"
)

var transitions = map[Status][]Status{
	Pending:    {Authorized, Denied},
	Authorized: {Captured, Canceled},
	Captured:   {Canceled, Settled, Chargebacked},
	Settled:    {Chargebacked},
}

var (
	ErrDebitInstallments = errors.New("débito não pode ser parcelado")
	ErrMaxInstallments   = errors.New("número de parcelas deve estar entre 1 e 12")
)

type Transaction struct {
	id                string
	merchantID        string
	amount            shared.Money
	product           shared.Product
	installments      int
	card              CardInfo
	status            Status
	authorizationCode string
	nsu               string
	responseCode      string
	createdAt         time.Time
	authorizedAt      *time.Time
	capturedAt        *time.Time
	canceledAt        *time.Time
	version           int
	events            []shared.Event
}

// New é a Factory: valida tudo o que a adquirente consegue validar ANTES de falar com o emissor.
func New(m *merchant.Merchant, amount shared.Money, product shared.Product, installments int, card CardInfo, now time.Time) (*Transaction, error) {
	if !m.IsActive() {
		return nil, merchant.ErrMerchantInactive
	}
	if !amount.IsPositive() {
		return nil, shared.ErrInvalidAmount
	}
	if installments < 1 || installments > 12 {
		return nil, ErrMaxInstallments
	}
	if product == shared.Debit && installments != 1 {
		return nil, ErrDebitInstallments
	}
	if _, err := m.FeePlan().MDR(product, installments); err != nil {
		return nil, err // ErrInstallmentsNotAllowed: o plano do EC não permite
	}
	if card.Token == "" || card.Last4 == "" {
		return nil, ErrInvalidToken
	}
	return &Transaction{
		id:           shared.NewID("tx"),
		merchantID:   m.ID(),
		amount:       amount,
		product:      product,
		installments: installments,
		card:         card,
		status:       Pending,
		createdAt:    now,
		version:      1,
	}, nil
}

func Restore(id, merchantID string, amount shared.Money, product shared.Product, installments int, card CardInfo,
	status Status, authorizationCode, nsu, responseCode string, createdAt time.Time,
	authorizedAt, capturedAt, canceledAt *time.Time, version int) *Transaction {
	return &Transaction{
		id: id, merchantID: merchantID, amount: amount, product: product, installments: installments, card: card,
		status: status, authorizationCode: authorizationCode, nsu: nsu, responseCode: responseCode,
		createdAt: createdAt, authorizedAt: authorizedAt, capturedAt: capturedAt, canceledAt: canceledAt, version: version,
	}
}

func (t *Transaction) transition(to Status) error {
	for _, allowed := range transitions[t.status] {
		if allowed == to {
			t.status = to
			t.version++
			return nil
		}
	}
	return fmt.Errorf("%w: %s → %s", shared.ErrInvalidTransition, t.status, to)
}

// Authorize registra a aprovação do emissor.
func (t *Transaction) Authorize(authorizationCode, nsu string, now time.Time) error {
	if err := t.transition(Authorized); err != nil {
		return err
	}
	t.authorizationCode = authorizationCode
	t.nsu = nsu
	t.responseCode = "00"
	t.authorizedAt = &now
	t.events = append(t.events, TransactionAuthorized{
		BaseEvent:  shared.NewBaseEvent("transaction.authorized", t.id, now),
		MerchantID: t.merchantID, Amount: t.amount, Product: t.product, Installments: t.installments,
		AuthorizationCode: authorizationCode, NSU: nsu, CardBrand: t.card.Brand, CardLast4: t.card.Last4,
	})
	return nil
}

// Deny registra a negativa (código de resposta do emissor, ex.: "51" saldo insuficiente).
func (t *Transaction) Deny(responseCode string, now time.Time) error {
	if err := t.transition(Denied); err != nil {
		return err
	}
	t.responseCode = responseCode
	t.events = append(t.events, TransactionDenied{
		BaseEvent:  shared.NewBaseEvent("transaction.denied", t.id, now),
		MerchantID: t.merchantID, Amount: t.amount, ResponseCode: responseCode,
	})
	return nil
}

// Capture confirma a venda. É a captura que gera recebíveis (via evento).
func (t *Transaction) Capture(now time.Time) error {
	if err := t.transition(Captured); err != nil {
		return err
	}
	t.capturedAt = &now
	t.events = append(t.events, TransactionCaptured{
		BaseEvent:  shared.NewBaseEvent("transaction.captured", t.id, now),
		MerchantID: t.merchantID, Amount: t.amount, Product: t.product, Installments: t.installments,
		CardBrand: t.card.Brand, CapturedAt: now,
	})
	return nil
}

// Cancel desfaz uma transação autorizada ou capturada (antes da liquidação).
func (t *Transaction) Cancel(now time.Time) error {
	if err := t.transition(Canceled); err != nil {
		return err
	}
	t.canceledAt = &now
	t.events = append(t.events, TransactionCanceled{
		BaseEvent:  shared.NewBaseEvent("transaction.canceled", t.id, now),
		MerchantID: t.merchantID, Amount: t.amount,
	})
	return nil
}

// MarkSettled é chamado quando o último recebível da transação é liquidado.
func (t *Transaction) MarkSettled() error { return t.transition(Settled) }

// Chargeback registra a contestação vinda do emissor.
func (t *Transaction) Chargeback(reasonCode string, now time.Time) error {
	if err := t.transition(Chargebacked); err != nil {
		return err
	}
	t.events = append(t.events, TransactionChargebacked{
		BaseEvent:  shared.NewBaseEvent("transaction.chargebacked", t.id, now),
		MerchantID: t.merchantID, Amount: t.amount, ReasonCode: reasonCode,
	})
	return nil
}

// PullEvents devolve os eventos acumulados e limpa a lista. O caso de uso chama
// depois de salvar, para gravar na outbox dentro da mesma transação de banco.
func (t *Transaction) PullEvents() []shared.Event {
	evs := t.events
	t.events = nil
	return evs
}

func (t *Transaction) ID() string                { return t.id }
func (t *Transaction) MerchantID() string        { return t.merchantID }
func (t *Transaction) Amount() shared.Money      { return t.amount }
func (t *Transaction) Product() shared.Product   { return t.product }
func (t *Transaction) Installments() int         { return t.installments }
func (t *Transaction) Card() CardInfo            { return t.card }
func (t *Transaction) Status() Status            { return t.status }
func (t *Transaction) AuthorizationCode() string { return t.authorizationCode }
func (t *Transaction) NSU() string               { return t.nsu }
func (t *Transaction) ResponseCode() string      { return t.responseCode }
func (t *Transaction) CreatedAt() time.Time      { return t.createdAt }
func (t *Transaction) AuthorizedAt() *time.Time  { return t.authorizedAt }
func (t *Transaction) CapturedAt() *time.Time    { return t.capturedAt }
func (t *Transaction) CanceledAt() *time.Time    { return t.canceledAt }
func (t *Transaction) Version() int              { return t.version }
