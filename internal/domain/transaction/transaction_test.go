package transaction

import (
	"errors"
	"testing"
	"time"

	"github.com/itallominatti/adquirente/internal/domain/merchant"
	"github.com/itallominatti/adquirente/internal/domain/shared"
)

var now = time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)

func activeMerchant(t *testing.T) *merchant.Merchant {
	t.Helper()
	doc, _ := merchant.NewDocument("11.222.333/0001-81")
	bank, _ := merchant.NewBankAccount("341", "0001", "12345-6", merchant.Checking)
	plan, _ := merchant.NewFeePlan(150, 250, map[int]shared.Bps{2: 350, 3: 350, 12: 450}, 200)
	m, err := merchant.New(doc, "Padaria", "5462", bank, plan, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Approve(now); err != nil {
		t.Fatal(err)
	}
	return m
}

func testCard(t *testing.T) CardInfo {
	t.Helper()
	c, err := CardInfoFromPAN("tok_abc", "4111111111111111")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCardInfoFromPAN(t *testing.T) {
	c := testCard(t)
	if c.Brand != "VISA" || c.BIN != "411111" || c.Last4 != "1111" {
		t.Fatalf("%+v", c)
	}
	if _, err := CardInfoFromPAN("tok", "4111111111111112"); !errors.Is(err, ErrInvalidPAN) {
		t.Error("Luhn inválido deveria falhar")
	}
	if MaskPAN("4111111111111111") != "411111******1111" {
		t.Error(MaskPAN("4111111111111111"))
	}
	if PAN("4111111111111111").String() != "411111******1111" {
		t.Error("PAN.String() deve mascarar")
	}
}

func TestNewValidation(t *testing.T) {
	m := activeMerchant(t)
	card := testCard(t)

	if _, err := New(m, 0, shared.Credit, 1, card, now); !errors.Is(err, shared.ErrInvalidAmount) {
		t.Error("valor zero")
	}
	if _, err := New(m, 1000, shared.Debit, 2, card, now); !errors.Is(err, ErrDebitInstallments) {
		t.Error("débito parcelado")
	}
	if _, err := New(m, 1000, shared.Credit, 7, card, now); !errors.Is(err, merchant.ErrInstallmentsNotAllowed) {
		t.Error("7x não está no plano")
	}
	if _, err := New(m, 1000, shared.Credit, 13, card, now); !errors.Is(err, ErrMaxInstallments) {
		t.Error("13x")
	}
	if err := m.Block(now); err != nil {
		t.Fatal(err)
	}
	if _, err := New(m, 1000, shared.Credit, 1, card, now); !errors.Is(err, merchant.ErrMerchantInactive) {
		t.Error("EC bloqueado")
	}
}

func TestStateMachine(t *testing.T) {
	m := activeMerchant(t)
	tx, err := New(m, 120000, shared.Credit, 12, testCard(t), now)
	if err != nil {
		t.Fatal(err)
	}
	if tx.Status() != Pending {
		t.Fatal("deve nascer PENDING")
	}
	// capturar antes de autorizar é impossível
	if err := tx.Capture(now); !errors.Is(err, shared.ErrInvalidTransition) {
		t.Fatal("capturar PENDING deveria falhar")
	}
	if err := tx.Authorize("123456", "000000123", now); err != nil {
		t.Fatal(err)
	}
	if err := tx.Capture(now); err != nil {
		t.Fatal(err)
	}
	if err := tx.Authorize("x", "y", now); !errors.Is(err, shared.ErrInvalidTransition) {
		t.Fatal("autorizar CAPTURED deveria falhar")
	}
	evs := tx.PullEvents()
	if len(evs) != 2 || evs[0].EventType() != "transaction.authorized" || evs[1].EventType() != "transaction.captured" {
		t.Fatalf("eventos: %+v", evs)
	}
	if len(tx.PullEvents()) != 0 {
		t.Fatal("PullEvents deve limpar")
	}
	if err := tx.MarkSettled(); err != nil {
		t.Fatal(err)
	}
	if err := tx.Cancel(now); !errors.Is(err, shared.ErrInvalidTransition) {
		t.Fatal("cancelar SETTLED deveria falhar")
	}
	if err := tx.Chargeback("4837", now); err != nil || tx.Status() != Chargebacked {
		t.Fatal("chargeback após liquidação deve ser permitido")
	}
}

func TestDenyAndCancel(t *testing.T) {
	m := activeMerchant(t)
	tx, _ := New(m, 5000, shared.Debit, 1, testCard(t), now)
	if err := tx.Deny("51", now); err != nil || tx.Status() != Denied || tx.ResponseCode() != "51" {
		t.Fatal("negativa")
	}
	if err := tx.Capture(now); !errors.Is(err, shared.ErrInvalidTransition) {
		t.Fatal("capturar negada")
	}

	tx2, _ := New(m, 5000, shared.Debit, 1, testCard(t), now)
	_ = tx2.Authorize("1", "2", now)
	if err := tx2.Cancel(now); err != nil || tx2.Status() != Canceled || tx2.CanceledAt() == nil {
		t.Fatal("cancelar autorizada")
	}
}
