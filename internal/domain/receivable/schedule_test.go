package receivable

import (
	"errors"
	"testing"
	"time"

	"github.com/itallominatti/adquirente/internal/domain/merchant"
	"github.com/itallominatti/adquirente/internal/domain/shared"
	"github.com/itallominatti/adquirente/internal/domain/transaction"
)

func date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func fixtures(t *testing.T) (*merchant.Merchant, transaction.CardInfo) {
	t.Helper()
	doc, _ := merchant.NewDocument("11.222.333/0001-81")
	bank, _ := merchant.NewBankAccount("341", "0001", "12345-6", merchant.Checking)
	plan, _ := merchant.NewFeePlan(150, 250, map[int]shared.Bps{3: 350, 12: 350}, 200)
	m, _ := merchant.New(doc, "Padaria", "5462", bank, plan, date(2026, 1, 1))
	_ = m.Approve(date(2026, 1, 1))
	card, _ := transaction.CardInfoFromPAN("tok_1", "4111111111111111")
	return m, card
}

func captured(t *testing.T, m *merchant.Merchant, card transaction.CardInfo, amount shared.Money, product shared.Product, n int, at time.Time) *transaction.Transaction {
	t.Helper()
	tx, err := transaction.New(m, amount, product, n, card, at)
	if err != nil {
		t.Fatal(err)
	}
	_ = tx.Authorize("123456", "1", at)
	_ = tx.Capture(at)
	return tx
}

func TestScheduleCredit12x(t *testing.T) {
	m, card := fixtures(t)
	cal := shared.BrazilCalendar{}
	tx := captured(t, m, card, 120000, shared.Credit, 12, date(2026, 3, 10))

	list, err := Schedule(tx, m.FeePlan(), cal, date(2026, 3, 10))
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 12 {
		t.Fatalf("len = %d", len(list))
	}
	var gross, fee, net shared.Money
	for i, r := range list {
		gross += r.Gross()
		fee += r.Fee()
		net += r.Net()
		if r.InstallmentNo() != i+1 || r.Status() != Scheduled || r.Net() != 9650 {
			t.Errorf("parcela %d: %+v", i+1, r)
		}
	}
	if gross != 120000 || fee != 4200 || net != 115800 {
		t.Fatalf("gross=%d fee=%d net=%d", gross, fee, net)
	}
	// parcela 1: 10/03 + 30 = 09/04 (quinta) → fica
	if !list[0].DueDate().Equal(date(2026, 4, 9)) {
		t.Errorf("parcela 1 vence %v", list[0].DueDate())
	}
	// parcela 2: 10/03 + 60 = 09/05 (sábado) → segunda 11/05
	if !list[1].DueDate().Equal(date(2026, 5, 11)) {
		t.Errorf("parcela 2 vence %v", list[1].DueDate())
	}
	// parcela 12: 10/03 + 360 = 05/03/2027 (sexta) → fica
	if !list[11].DueDate().Equal(date(2027, 3, 5)) {
		t.Errorf("parcela 12 vence %v", list[11].DueDate())
	}
}

func TestScheduleDebitAndRounding(t *testing.T) {
	m, card := fixtures(t)
	cal := shared.BrazilCalendar{}

	// débito sexta 04/09/2026 → D+1 útil pula fim de semana e 07/09 → 08/09
	tx := captured(t, m, card, 10000, shared.Debit, 1, date(2026, 9, 4))
	list, err := Schedule(tx, m.FeePlan(), cal, date(2026, 9, 4))
	if err != nil || len(list) != 1 {
		t.Fatal(err)
	}
	if !list[0].DueDate().Equal(date(2026, 9, 8)) || list[0].Fee() != 150 || list[0].Net() != 9850 {
		t.Fatalf("%+v", list[0])
	}

	// R$ 10,00 em 3x: bruto 334/333/333; fee 2,5%*... MDR 3x = 350 bps → fee 35 → 13/11/11; net 321/322/322
	tx3 := captured(t, m, card, 1000, shared.Credit, 3, date(2026, 9, 4))
	list3, _ := Schedule(tx3, m.FeePlan(), cal, date(2026, 9, 4))
	wantGross := []shared.Money{334, 333, 333}
	wantFee := []shared.Money{13, 11, 11}
	var net shared.Money
	for i, r := range list3 {
		if r.Gross() != wantGross[i] || r.Fee() != wantFee[i] {
			t.Errorf("parcela %d gross=%d fee=%d", i+1, r.Gross(), r.Fee())
		}
		net += r.Net()
	}
	if net != 965 {
		t.Fatalf("net total = %d", net)
	}
}

func TestScheduleRequiresCapture(t *testing.T) {
	m, card := fixtures(t)
	tx, _ := transaction.New(m, 1000, shared.Credit, 1, card, date(2026, 9, 4))
	_ = tx.Authorize("1", "2", date(2026, 9, 4))
	if _, err := Schedule(tx, m.FeePlan(), shared.BrazilCalendar{}, date(2026, 9, 4)); !errors.Is(err, ErrNotCaptured) {
		t.Fatal("autorizada sem captura não gera agenda")
	}
}

func TestReceivableTransitionsAndUnits(t *testing.T) {
	m, card := fixtures(t)
	tx := captured(t, m, card, 30000, shared.Credit, 3, date(2026, 9, 4))
	list, _ := Schedule(tx, m.FeePlan(), shared.BrazilCalendar{}, date(2026, 9, 4))
	now := date(2026, 9, 5)

	r := list[0]
	if !r.IsAnticipable(now) || r.DaysUntilDue(now) <= 0 {
		t.Fatal("parcela futura deve ser antecipável")
	}
	if err := r.Anticipate("ant_1", now); err != nil || r.Status() != Anticipated || r.IsAnticipable(now) {
		t.Fatal("antecipar")
	}
	if err := r.Cancel(now); !errors.Is(err, shared.ErrInvalidTransition) {
		t.Fatal("cancelar antecipado deve falhar")
	}
	if err := r.Settle("stl_1", now); err != nil || r.SettlementID() != "stl_1" {
		t.Fatal("liquidar antecipado (dinheiro fica com a adquirente)")
	}
	if err := list[1].Chargeback(now); err != nil {
		t.Fatal(err)
	}

	units := GroupUnits(list)
	if len(units) != 3 { // 3 datas diferentes, mesma bandeira/produto
		t.Fatalf("units = %d", len(units))
	}
	ev := NewReceivablesScheduled(list, now)
	if ev.Count != 3 || ev.GrossTotal != 30000 || ev.EventType() != "receivable.scheduled" {
		t.Fatalf("%+v", ev)
	}
}
