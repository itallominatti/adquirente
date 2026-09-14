package anticipation

import (
	"errors"
	"testing"
	"time"

	"github.com/itallominatti/adquirente/internal/domain/receivable"
	"github.com/itallominatti/adquirente/internal/domain/shared"
)

func date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func TestCompoundPricer(t *testing.T) {
	p := CompoundPricer{}
	cases := []struct {
		days int
		want shared.Money
	}{
		{30, 9461}, {60, 9275}, {90, 9093}, {120, 8915}, {150, 8740}, {180, 8569},
		{210, 8401}, {240, 8236}, {270, 8075}, {300, 7916}, {330, 7761}, {360, 7609},
	}
	var total shared.Money
	for _, c := range cases {
		got := p.PresentValue(9650, c.days, 200)
		if got != c.want {
			t.Errorf("PV(96,50, %d dias) = %d want %d", c.days, got, c.want)
		}
		total += got
	}
	if total != 102051 { // R$ 1.020,51 hoje por R$ 1.158,00 ao longo do ano
		t.Errorf("total = %d", total)
	}
	if p.PresentValue(1000, 0, 200) != 1000 || p.PresentValue(1000, 30, 0) != 1000 {
		t.Error("sem prazo ou sem taxa, não há desconto")
	}
}

func scheduledReceivable(merchantID string, net shared.Money, due time.Time) *receivable.Receivable {
	return receivable.Restore(shared.NewID("rcv"), "tx_1", merchantID, 1, 1, shared.Credit, "VISA",
		net, 0, net, due, receivable.Scheduled, "", "", 1, due, due)
}

func TestSimulateAndConfirm(t *testing.T) {
	today := date(2026, 9, 14)
	rs := []*receivable.Receivable{
		scheduledReceivable("m_1", 9650, date(2026, 10, 14)), // 30 dias
		scheduledReceivable("m_1", 9650, date(2026, 11, 13)), // 60 dias
	}
	a, err := Simulate("m_1", rs, today, 200, CompoundPricer{})
	if err != nil {
		t.Fatal(err)
	}
	if a.Gross() != 19300 || a.Net() != 9461+9275 || a.Discount() != 19300-(9461+9275) || a.Status() != Simulated {
		t.Fatalf("gross=%d net=%d discount=%d", a.Gross(), a.Net(), a.Discount())
	}
	if err := a.Confirm(rs, today); err != nil {
		t.Fatal(err)
	}
	if a.Status() != Confirmed || rs[0].Status() != receivable.Anticipated || rs[0].AnticipationID() != a.ID() {
		t.Fatal("confirmar deve marcar os recebíveis")
	}
	evs := a.PullEvents()
	if len(evs) != 1 || evs[0].EventType() != "receivable.anticipated" {
		t.Fatalf("%+v", evs)
	}
	if err := a.Confirm(rs, today); !errors.Is(err, ErrAlreadyDone) {
		t.Fatal("confirmar duas vezes")
	}
}

func TestSimulateRejects(t *testing.T) {
	today := date(2026, 9, 14)
	if _, err := Simulate("m_1", nil, today, 200, CompoundPricer{}); !errors.Is(err, ErrNoReceivables) {
		t.Error("lista vazia")
	}
	other := scheduledReceivable("m_2", 100, date(2026, 10, 1))
	if _, err := Simulate("m_1", []*receivable.Receivable{other}, today, 200, CompoundPricer{}); !errors.Is(err, ErrWrongMerchant) {
		t.Error("recebível de outro EC")
	}
	past := scheduledReceivable("m_1", 100, date(2026, 9, 14)) // vence hoje: não antecipa
	if _, err := Simulate("m_1", []*receivable.Receivable{past}, today, 200, CompoundPricer{}); !errors.Is(err, ErrNotAnticipable) {
		t.Error("vence hoje")
	}
	done := scheduledReceivable("m_1", 100, date(2026, 10, 1))
	_ = done.Anticipate("ant_x", today)
	if _, err := Simulate("m_1", []*receivable.Receivable{done}, today, 200, CompoundPricer{}); !errors.Is(err, ErrNotAnticipable) {
		t.Error("já antecipado")
	}
}
