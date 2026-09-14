package settlement

import (
	"errors"
	"testing"
	"time"

	"github.com/itallominatti/adquirente/internal/domain/merchant"
	"github.com/itallominatti/adquirente/internal/domain/receivable"
	"github.com/itallominatti/adquirente/internal/domain/shared"
)

func date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func testMerchant(t *testing.T) *merchant.Merchant {
	t.Helper()
	doc, _ := merchant.NewDocument("11.222.333/0001-81")
	bank, _ := merchant.NewBankAccount("341", "0001", "12345-6", merchant.Checking)
	plan, _ := merchant.NewFeePlan(150, 250, nil, 200)
	m, _ := merchant.New(doc, "Padaria", "5462", bank, plan, date(2026, 1, 1))
	_ = m.Approve(date(2026, 1, 1))
	return m
}

func rcv(merchantID string, net shared.Money, due time.Time, status receivable.Status) *receivable.Receivable {
	return receivable.Restore(shared.NewID("rcv"), "tx", merchantID, 1, 1, shared.Credit, "VISA", net, 0, net, due, status, "", "", 1, due, due)
}

func TestBuildWithLiensAndAnticipated(t *testing.T) {
	m := testMerchant(t)
	day := date(2026, 10, 13)
	rs := []*receivable.Receivable{
		rcv(m.ID(), 50000, day, receivable.Scheduled),
		rcv(m.ID(), 30000, day, receivable.Scheduled),
		rcv(m.ID(), 20000, day, receivable.Anticipated), // já pago ao EC na antecipação
	}
	creditor, _ := merchant.NewBankAccount("001", "1234", "99999-9", merchant.Checking)
	liens := []receivable.Lien{{MerchantID: m.ID(), DueDate: day, Amount: 25000, CreditorName: "Banco Credor", Creditor: creditor}}

	s, err := Build(day, m, rs, liens, day)
	if err != nil {
		t.Fatal(err)
	}
	if s.Total() != 100000 || s.Retained() != 20000 || s.ToCreditors() != 25000 || s.ToMerchant() != 55000 {
		t.Fatalf("total=%d retained=%d creditors=%d merchant=%d", s.Total(), s.Retained(), s.ToCreditors(), s.ToMerchant())
	}
	if len(s.Orders()) != 2 || s.Orders()[0].Kind != ToCreditor || s.Orders()[1].Kind != ToMerchant {
		t.Fatalf("orders: %+v", s.Orders())
	}
	if s.Orders()[1].BankAccount != m.BankAccount() {
		t.Fatal("ordem do EC deve ir para o domicílio bancário dele")
	}

	// ônus maior que o disponível: paga só o que há, nada vai para o EC
	big := []receivable.Lien{{MerchantID: m.ID(), DueDate: day, Amount: 999999, CreditorName: "Banco", Creditor: creditor}}
	s2, _ := Build(day, m, rs, big, day)
	if s2.ToCreditors() != 80000 || s2.ToMerchant() != 0 || len(s2.Orders()) != 1 {
		t.Fatalf("creditors=%d merchant=%d", s2.ToCreditors(), s2.ToMerchant())
	}
}

func TestBuildRejects(t *testing.T) {
	m := testMerchant(t)
	day := date(2026, 10, 13)
	if _, err := Build(day, m, nil, nil, day); !errors.Is(err, ErrNothingToSettle) {
		t.Error("vazio")
	}
	if _, err := Build(day, m, []*receivable.Receivable{rcv("m_other", 1, day, receivable.Scheduled)}, nil, day); !errors.Is(err, ErrWrongMerchant) {
		t.Error("outro EC")
	}
	if _, err := Build(day, m, []*receivable.Receivable{rcv(m.ID(), 1, day.AddDate(0, 0, 1), receivable.Scheduled)}, nil, day); !errors.Is(err, ErrWrongDay) {
		t.Error("outro dia")
	}
	if _, err := Build(day, m, []*receivable.Receivable{rcv(m.ID(), 1, day, receivable.Settled)}, nil, day); !errors.Is(err, ErrWrongStatus) {
		t.Error("já liquidado")
	}
}

func TestLifecycle(t *testing.T) {
	m := testMerchant(t)
	day := date(2026, 10, 13)
	s, _ := Build(day, m, []*receivable.Receivable{rcv(m.ID(), 100, day, receivable.Scheduled)}, nil, day)
	if err := s.MarkConfirmed(day); !errors.Is(err, shared.ErrInvalidTransition) {
		t.Fatal("confirmar sem enviar")
	}
	_ = s.MarkSent("ARQ-0001", day)
	if err := s.MarkConfirmed(day); err != nil || s.Status() != Confirmed {
		t.Fatal(err)
	}
	if evs := s.PullEvents(); len(evs) != 1 || evs[0].EventType() != "settlement.completed" {
		t.Fatalf("%+v", evs)
	}
	s2, _ := Build(day, m, []*receivable.Receivable{rcv(m.ID(), 100, day, receivable.Scheduled)}, nil, day)
	_ = s2.MarkSent("ARQ-0002", day)
	if err := s2.MarkFailed("conta inválida", day); err != nil || s2.FailureReason() == "" {
		t.Fatal(err)
	}
}
