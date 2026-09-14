package merchant

import (
	"errors"
	"testing"
	"time"

	"github.com/itallominatti/adquirente/internal/domain/shared"
)

func TestNewDocument(t *testing.T) {
	cases := []struct {
		in    string
		valid bool
	}{
		{"11.222.333/0001-81", true},
		{"11222333000181", true},
		{"11.222.333/0001-80", false}, // dígito verificador errado
		{"00000000000000", false},     // todos iguais
		{"123", false},
		{"11.222.333/0001-8A", false}, // verificador não pode ser letra
	}
	for _, c := range cases {
		_, err := NewDocument(c.in)
		if (err == nil) != c.valid {
			t.Errorf("NewDocument(%q): err=%v, valid=%v", c.in, err, c.valid)
		}
	}
	d, _ := NewDocument("11.222.333/0001-81")
	if d.Masked() != "**.***.***/0001-81" {
		t.Errorf("Masked() = %q", d.Masked())
	}
}

func testPlan(t *testing.T) FeePlan {
	t.Helper()
	plan, err := NewFeePlan(150, 250, map[int]shared.Bps{2: 350, 3: 350, 6: 350, 12: 450}, 200)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func TestFeePlanMDR(t *testing.T) {
	plan := testPlan(t)
	cases := []struct {
		product      shared.Product
		installments int
		want         shared.Bps
		wantErr      bool
	}{
		{shared.Debit, 1, 150, false},
		{shared.Credit, 1, 250, false},
		{shared.Credit, 6, 350, false},
		{shared.Credit, 12, 450, false},
		{shared.Credit, 7, 0, true},  // não está no plano
		{shared.Debit, 2, 0, true},   // débito não parcela
		{shared.Credit, 13, 0, true}, // acima do máximo
	}
	for _, c := range cases {
		got, err := plan.MDR(c.product, c.installments)
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("MDR(%s,%d) = %d, %v; want %d, err=%v", c.product, c.installments, got, err, c.want, c.wantErr)
		}
	}
}

func newTestMerchant(t *testing.T) *Merchant {
	t.Helper()
	doc, _ := NewDocument("11.222.333/0001-81")
	bank, _ := NewBankAccount("341", "0001", "12345-6", Checking)
	m, err := New(doc, "Padaria Pão Quente LTDA", "5462", bank, testPlan(t), time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestMerchantLifecycle(t *testing.T) {
	m := newTestMerchant(t)
	now := time.Date(2026, 9, 2, 9, 0, 0, 0, time.UTC)

	if m.Status() != UnderReview || m.IsActive() {
		t.Fatal("EC deve nascer em análise")
	}
	if err := m.Block(now); !errors.Is(err, shared.ErrInvalidTransition) {
		t.Fatalf("bloquear em análise deve falhar, err=%v", err)
	}
	if err := m.Approve(now); err != nil {
		t.Fatal(err)
	}
	if !m.IsActive() || m.Version() != 2 {
		t.Fatalf("status=%s version=%d", m.Status(), m.Version())
	}
	if err := m.Approve(now); !errors.Is(err, shared.ErrInvalidTransition) {
		t.Fatal("aprovar duas vezes deve falhar")
	}
	if err := m.Block(now); err != nil || m.IsActive() {
		t.Fatal("bloqueio falhou")
	}
}

func TestMerchantWebhookAntiSSRF(t *testing.T) {
	m := newTestMerchant(t)
	now := time.Now()
	for _, bad := range []string{"http://loja.com/hook", "https://localhost/hook", "https://10.0.0.5/hook", "https://169.254.169.254/latest", "ftp://x"} {
		if err := m.SetWebhook(bad, now); !errors.Is(err, ErrInvalidWebhook) {
			t.Errorf("%q deveria ser rejeitada", bad)
		}
	}
	if err := m.SetWebhook("https://loja.com.br/webhooks/adquirente", now); err != nil {
		t.Fatal(err)
	}
}

func TestNewMerchantValidation(t *testing.T) {
	doc, _ := NewDocument("11.222.333/0001-81")
	bank, _ := NewBankAccount("341", "0001", "12345-6", Checking)
	plan := testPlan(t)
	if _, err := New(doc, "  ", "5462", bank, plan, time.Now()); !errors.Is(err, ErrInvalidLegalName) {
		t.Error("razão social vazia deveria falhar")
	}
	if _, err := New(doc, "X", "54", bank, plan, time.Now()); !errors.Is(err, ErrInvalidMCC) {
		t.Error("MCC curto deveria falhar")
	}
	if _, err := New(doc, "X", "5462", BankAccount{}, plan, time.Now()); !errors.Is(err, ErrInvalidBankAccount) {
		t.Error("conta vazia deveria falhar")
	}
	if _, err := NewBankAccount("34", "0001", "1", Checking); !errors.Is(err, ErrInvalidBankAccount) {
		t.Error("código de banco curto deveria falhar")
	}
}
