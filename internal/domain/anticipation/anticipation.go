package anticipation

import (
	"errors"
	"fmt"
	"time"

	"github.com/itallominatti/adquirente/internal/domain/receivable"
	"github.com/itallominatti/adquirente/internal/domain/shared"
)

type Status string

const (
	Simulated Status = "SIMULATED" // só cálculo; nada mudou
	Confirmed Status = "CONFIRMED" // recebíveis marcados, dinheiro devido ao EC
)

var (
	ErrNoReceivables  = errors.New("nenhum recebível informado")
	ErrNotAnticipable = errors.New("recebível não está elegível para antecipação")
	ErrWrongMerchant  = errors.New("recebível não pertence ao estabelecimento")
	ErrAlreadyDone    = errors.New("antecipação já confirmada")
)

type Item struct {
	ReceivableID string       `json:"receivable_id"`
	DueDate      time.Time    `json:"due_date"`
	Days         int          `json:"days"`
	Net          shared.Money `json:"net"`           // o que venceria
	PresentValue shared.Money `json:"present_value"` // o que o EC recebe hoje
	Discount     shared.Money `json:"discount"`      // Net - PresentValue
}

type Anticipation struct {
	id          string
	merchantID  string
	items       []Item
	gross       shared.Money // soma dos Net dos recebíveis
	discount    shared.Money // receita da adquirente
	net         shared.Money // o que o EC recebe
	rateMonth   shared.Bps
	requestedAt time.Time
	status      Status
	version     int
	events      []shared.Event
}

func Simulate(merchantID string, receivables []*receivable.Receivable, today time.Time, rateMonth shared.Bps, pricer Pricer) (*Anticipation, error) {
	if len(receivables) == 0 {
		return nil, ErrNoReceivables
	}
	a := &Anticipation{id: shared.NewID("ant"), merchantID: merchantID, rateMonth: rateMonth, requestedAt: today, status: Simulated, version: 1}
	for _, r := range receivables {
		if r.MerchantID() != merchantID {
			return nil, fmt.Errorf("%w: %s", ErrWrongMerchant, r.ID())
		}
		if !r.IsAnticipable(today) {
			return nil, fmt.Errorf("%w: %s (%s, vence %s)", ErrNotAnticipable, r.ID(), r.Status(), r.DueDate().Format("2006-01-02"))
		}
		days := r.DaysUntilDue(today)
		pv := pricer.PresentValue(r.Net(), days, rateMonth)
		a.items = append(a.items, Item{ReceivableID: r.ID(), DueDate: r.DueDate(), Days: days, Net: r.Net(), PresentValue: pv, Discount: r.Net() - pv})
		a.gross += r.Net()
		a.net += pv
	}
	a.discount = a.gross - a.net
	return a, nil
}

func (a *Anticipation) Confirm(receivables []*receivable.Receivable, now time.Time) error {
	if a.status == Confirmed {
		return ErrAlreadyDone
	}
	if len(receivables) != len(a.items) {
		return ErrNotAnticipable
	}
	ids := make([]string, 0, len(receivables))
	for _, r := range receivables {
		if err := r.Anticipate(a.id, now); err != nil {
			return err
		}
		ids = append(ids, r.ID())
	}
	a.status = Confirmed
	a.version++
	a.events = append(a.events, AnticipationConfirmed{
		BaseEvent:      shared.NewBaseEvent("receivable.anticipated", a.id, now),
		AnticipationID: a.id, MerchantID: a.merchantID, Gross: a.gross, Discount: a.discount, Net: a.net, ReceivableIDs: ids,
	})
	return nil
}

func Restore(id, merchantID string, items []Item, gross, discount, net shared.Money, rateMonth shared.Bps, requestedAt time.Time, status Status, version int) *Anticipation {
	return &Anticipation{id: id, merchantID: merchantID, items: items, gross: gross, discount: discount, net: net, rateMonth: rateMonth, requestedAt: requestedAt, status: status, version: version}
}

func (a *Anticipation) PullEvents() []shared.Event { evs := a.events; a.events = nil; return evs }

func (a *Anticipation) ID() string             { return a.id }
func (a *Anticipation) MerchantID() string     { return a.merchantID }
func (a *Anticipation) Items() []Item          { return a.items }
func (a *Anticipation) Gross() shared.Money    { return a.gross }
func (a *Anticipation) Discount() shared.Money { return a.discount }
func (a *Anticipation) Net() shared.Money      { return a.net }
func (a *Anticipation) RateMonth() shared.Bps  { return a.rateMonth }
func (a *Anticipation) RequestedAt() time.Time { return a.requestedAt }
func (a *Anticipation) Status() Status         { return a.status }
func (a *Anticipation) Version() int           { return a.version }

func (a *Anticipation) ReceivableIDs() []string {
	ids := make([]string, len(a.items))
	for i, it := range a.items {
		ids[i] = it.ReceivableID
	}
	return ids
}

type AnticipationConfirmed struct {
	shared.BaseEvent
	AnticipationID string       `json:"anticipation_id"`
	MerchantID     string       `json:"merchant_id"`
	Gross          shared.Money `json:"gross"`
	Discount       shared.Money `json:"discount"`
	Net            shared.Money `json:"net"`
	ReceivableIDs  []string     `json:"receivable_ids"`
}
