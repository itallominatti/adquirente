package transaction

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/itallominatti/adquirente/internal/application"
	"github.com/itallominatti/adquirente/internal/domain/merchant"
	"github.com/itallominatti/adquirente/internal/domain/shared"
	"github.com/itallominatti/adquirente/internal/domain/transaction"
)

// ---- fakes: como as interfaces são pequenas, escrevemos à mão (Parte 3.7) ----

type fakeMerchants struct{ m *merchant.Merchant }

func (f fakeMerchants) Create(context.Context, *merchant.Merchant) error { return nil }
func (f fakeMerchants) Update(context.Context, *merchant.Merchant) error { return nil }
func (f fakeMerchants) FindByID(_ context.Context, id string) (*merchant.Merchant, error) {
	if f.m != nil && f.m.ID() == id {
		return f.m, nil
	}
	return nil, shared.ErrNotFound
}

type fakeVault struct{}

func (fakeVault) Detokenize(_ context.Context, token string) (transaction.CardData, error) {
	if token == "tok_visa" {
		return transaction.CardData{PAN: "4111111111111111", Expiry: "12/30"}, nil
	}
	return transaction.CardData{}, transaction.ErrInvalidToken
}

type fakeIssuer struct {
	approve  bool
	code     string
	err      error
	reversed []string
}

func (f *fakeIssuer) Authorize(ctx context.Context, _ transaction.AuthorizationRequest) (transaction.AuthorizationResponse, error) {
	if f.err != nil {
		return transaction.AuthorizationResponse{}, f.err
	}
	if f.approve {
		return transaction.AuthorizationResponse{Approved: true, AuthorizationCode: "123456", ResponseCode: "00"}, nil
	}
	return transaction.AuthorizationResponse{Approved: false, ResponseCode: f.code}, nil
}
func (f *fakeIssuer) Reverse(_ context.Context, id string) error {
	f.reversed = append(f.reversed, id)
	return nil
}

type memTxRepo struct {
	saved map[string]*transaction.Transaction
}

func (r *memTxRepo) Create(_ context.Context, t *transaction.Transaction) error {
	r.saved[t.ID()] = t
	return nil
}
func (r *memTxRepo) Update(_ context.Context, t *transaction.Transaction) error {
	r.saved[t.ID()] = t
	return nil
}
func (r *memTxRepo) FindByID(_ context.Context, merchantID, id string) (*transaction.Transaction, error) {
	t, ok := r.saved[id]
	if !ok || t.MerchantID() != merchantID {
		return nil, shared.ErrNotFound
	}
	return t, nil
}
func (r *memTxRepo) FindByIDForUpdate(ctx context.Context, merchantID, id string) (*transaction.Transaction, error) {
	return r.FindByID(ctx, merchantID, id)
}

type memOutbox struct{ events []shared.Event }

func (o *memOutbox) Append(_ context.Context, evs ...shared.Event) error {
	o.events = append(o.events, evs...)
	return nil
}

type seqNSU struct{ n int }

func (s *seqNSU) Next(context.Context) (string, error) {
	s.n++
	return "NSU" + string(rune('0'+s.n)), nil
}

type fakeUoW struct{ repos application.Repositories }

func (u fakeUoW) Do(_ context.Context, fn func(application.Repositories) error) error {
	return fn(u.repos)
}

// ---- fixtures ----

var now = time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)

func activeMerchant(t *testing.T) *merchant.Merchant {
	t.Helper()
	doc, _ := merchant.NewDocument("11.222.333/0001-81")
	bank, _ := merchant.NewBankAccount("341", "0001", "12345-6", merchant.Checking)
	plan, _ := merchant.NewFeePlan(150, 250, map[int]shared.Bps{12: 450}, 200)
	m, _ := merchant.New(doc, "Padaria", "5462", bank, plan, now)
	_ = m.Approve(now)
	return m
}

func setup(t *testing.T, issuer *fakeIssuer, rules ...Rule) (*Authorize, *memTxRepo, *memOutbox, *merchant.Merchant) {
	t.Helper()
	m := activeMerchant(t)
	txRepo := &memTxRepo{saved: map[string]*transaction.Transaction{}}
	outbox := &memOutbox{}
	uow := fakeUoW{repos: application.Repositories{Transactions: txRepo, Outbox: outbox, NSU: &seqNSU{}}}
	uc := NewAuthorize(fakeMerchants{m}, fakeVault{}, issuer, uow, shared.FixedClock{T: now}, 2*time.Second, rules...)
	return uc, txRepo, outbox, m
}

func TestAuthorizeApproved(t *testing.T) {
	uc, repo, outbox, m := setup(t, &fakeIssuer{approve: true})
	tx, err := uc.Execute(context.Background(), AuthorizeCommand{MerchantID: m.ID(), Amount: 120000, Product: "CREDIT", Installments: 12, CardToken: "tok_visa"})
	if err != nil {
		t.Fatal(err)
	}
	if tx.Status() != transaction.Authorized || tx.AuthorizationCode() != "123456" || tx.NSU() == "" {
		t.Fatalf("%+v", tx)
	}
	if tx.Card().Last4 != "1111" || tx.Card().Brand != "VISA" {
		t.Fatal("dados públicos do cartão")
	}
	if len(repo.saved) != 1 || len(outbox.events) != 1 || outbox.events[0].EventType() != "transaction.authorized" {
		t.Fatal("deve salvar transação e evento juntos")
	}
}

func TestAuthorizeDeniedIsNotAnError(t *testing.T) {
	uc, repo, outbox, m := setup(t, &fakeIssuer{approve: false, code: "51"})
	tx, err := uc.Execute(context.Background(), AuthorizeCommand{MerchantID: m.ID(), Amount: 5000, Product: "DEBIT", Installments: 1, CardToken: "tok_visa"})
	if err != nil {
		t.Fatal(err)
	}
	if tx.Status() != transaction.Denied || tx.ResponseCode() != "51" {
		t.Fatalf("%+v", tx)
	}
	if len(repo.saved) != 1 || outbox.events[0].EventType() != "transaction.denied" {
		t.Fatal("negada também é persistida (auditoria)")
	}
}

func TestAuthorizeTimeoutTriggersReversal(t *testing.T) {
	issuer := &fakeIssuer{err: context.DeadlineExceeded}
	uc, _, _, m := setup(t, issuer)
	tx, err := uc.Execute(context.Background(), AuthorizeCommand{MerchantID: m.ID(), Amount: 5000, Product: "DEBIT", Installments: 1, CardToken: "tok_visa"})
	if err != nil {
		t.Fatal(err)
	}
	if tx.Status() != transaction.Denied || tx.ResponseCode() != "91" {
		t.Fatalf("timeout deve virar negada 91, got %s/%s", tx.Status(), tx.ResponseCode())
	}
	if len(issuer.reversed) != 1 || issuer.reversed[0] != tx.ID() {
		t.Fatal("reversal deve ser enviado")
	}
}

func TestAuthorizeIssuerDownIsError(t *testing.T) {
	uc, repo, _, m := setup(t, &fakeIssuer{err: errors.New("connection refused")})
	_, err := uc.Execute(context.Background(), AuthorizeCommand{MerchantID: m.ID(), Amount: 5000, Product: "DEBIT", Installments: 1, CardToken: "tok_visa"})
	if !errors.Is(err, ErrIssuerUnavailable) {
		t.Fatalf("err = %v", err)
	}
	if len(repo.saved) != 0 {
		t.Fatal("falha técnica não deve persistir nada")
	}
}

func TestAuthorizeValidationAndRules(t *testing.T) {
	uc, _, _, m := setup(t, &fakeIssuer{approve: true}, MaxAmountRule(100000))
	cases := []struct {
		name string
		cmd  AuthorizeCommand
		want error
	}{
		{"EC inexistente", AuthorizeCommand{MerchantID: "m_x", Amount: 100, Product: "DEBIT", Installments: 1, CardToken: "tok_visa"}, shared.ErrNotFound},
		{"produto inválido", AuthorizeCommand{MerchantID: m.ID(), Amount: 100, Product: "PIX", Installments: 1, CardToken: "tok_visa"}, shared.ErrInvalidProduct},
		{"token inválido", AuthorizeCommand{MerchantID: m.ID(), Amount: 100, Product: "DEBIT", Installments: 1, CardToken: "tok_x"}, transaction.ErrInvalidToken},
		{"parcelas fora do plano", AuthorizeCommand{MerchantID: m.ID(), Amount: 100, Product: "CREDIT", Installments: 6, CardToken: "tok_visa"}, merchant.ErrInstallmentsNotAllowed},
		{"acima do limite", AuthorizeCommand{MerchantID: m.ID(), Amount: 100001, Product: "CREDIT", Installments: 1, CardToken: "tok_visa"}, ErrLimitExceeded},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := uc.Execute(context.Background(), c.cmd)
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
		})
	}
}

func TestCaptureAndCancel(t *testing.T) {
	uc, repo, outbox, m := setup(t, &fakeIssuer{approve: true})
	tx, _ := uc.Execute(context.Background(), AuthorizeCommand{MerchantID: m.ID(), Amount: 5000, Product: "DEBIT", Installments: 1, CardToken: "tok_visa"})

	lc := NewLifecycle(fakeUoW{repos: application.Repositories{Transactions: repo, Outbox: outbox}}, shared.FixedClock{T: now})
	if _, err := lc.Capture(context.Background(), "m_other", tx.ID()); !errors.Is(err, shared.ErrNotFound) {
		t.Fatal("outro EC não enxerga a transação")
	}
	got, err := lc.Capture(context.Background(), m.ID(), tx.ID())
	if err != nil || got.Status() != transaction.Captured {
		t.Fatal(err)
	}
	if _, err := lc.Capture(context.Background(), m.ID(), tx.ID()); !errors.Is(err, shared.ErrInvalidTransition) {
		t.Fatal("capturar duas vezes")
	}
	if _, err := lc.Cancel(context.Background(), m.ID(), tx.ID()); err != nil {
		t.Fatal(err)
	}
	if len(outbox.events) != 3 { // authorized, captured, canceled
		t.Fatalf("eventos = %d", len(outbox.events))
	}
}
