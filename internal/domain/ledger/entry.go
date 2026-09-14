package ledger

import (
	"context"
	"time"

	"github.com/itallominatti/adquirente/internal/domain/anticipation"
	"github.com/itallominatti/adquirente/internal/domain/receivable"
	"github.com/itallominatti/adquirente/internal/domain/settlement"
	"github.com/itallominatti/adquirente/internal/domain/shared"
)

type Account string

const (
	ReceivableFromIssuer Account = "receivable_from_issuer" // o que o emissor nos deve (ativo)
	PayableToMerchant    Account = "payable_to_merchant"    // o que devemos ao EC (passivo)
	RevenueFees          Account = "revenue_fees"           // receita de MDR
	RevenueAnticipation  Account = "revenue_anticipation"   // receita de antecipação
	BankOutgoing         Account = "bank_outgoing"          // dinheiro que saiu da nossa conta
	ChargebackLoss       Account = "chargeback_loss"        // perdas com contestação
)

type Entry struct {
	ID         string
	Account    Account
	MerchantID string
	Debit      shared.Money
	Credit     shared.Money
	RefType    string // "receivable", "settlement", "anticipation", "chargeback"
	RefID      string
	At         time.Time
}

func debit(acc Account, merchantID string, v shared.Money, refType, refID string, at time.Time) Entry {
	return Entry{ID: shared.NewID("le"), Account: acc, MerchantID: merchantID, Debit: v, RefType: refType, RefID: refID, At: at}
}

func credit(acc Account, merchantID string, v shared.Money, refType, refID string, at time.Time) Entry {
	return Entry{ID: shared.NewID("le"), Account: acc, MerchantID: merchantID, Credit: v, RefType: refType, RefID: refID, At: at}
}

func ForScheduled(list []*receivable.Receivable, at time.Time) []Entry {
	var out []Entry
	for _, r := range list {
		out = append(out,
			debit(ReceivableFromIssuer, r.MerchantID(), r.Gross(), "receivable", r.ID(), at),
			credit(PayableToMerchant, r.MerchantID(), r.Net(), "receivable", r.ID(), at),
			credit(RevenueFees, r.MerchantID(), r.Fee(), "receivable", r.ID(), at),
		)
	}
	return out
}

func ForAnticipation(a *anticipation.Anticipation, at time.Time) []Entry {
	return []Entry{
		debit(PayableToMerchant, a.MerchantID(), a.Gross(), "anticipation", a.ID(), at),
		credit(BankOutgoing, a.MerchantID(), a.Net(), "anticipation", a.ID(), at),
		credit(RevenueAnticipation, a.MerchantID(), a.Discount(), "anticipation", a.ID(), at),
	}
}

func ForSettlement(s *settlement.Settlement, at time.Time) []Entry {
	paid := s.ToMerchant() + s.ToCreditors()
	if paid == 0 {
		return nil
	}
	return []Entry{
		debit(PayableToMerchant, s.MerchantID(), paid, "settlement", s.ID(), at),
		credit(BankOutgoing, s.MerchantID(), paid, "settlement", s.ID(), at),
	}
}

func ForChargeback(r *receivable.Receivable, at time.Time) []Entry {
	return []Entry{
		credit(ReceivableFromIssuer, r.MerchantID(), r.Gross(), "chargeback", r.ID(), at),
		debit(PayableToMerchant, r.MerchantID(), r.Net(), "chargeback", r.ID(), at),
		debit(RevenueFees, r.MerchantID(), r.Fee(), "chargeback", r.ID(), at),
	}
}

func ForChargebackAnticipated(r *receivable.Receivable, at time.Time) []Entry {
	return []Entry{
		credit(ReceivableFromIssuer, r.MerchantID(), r.Gross(), "chargeback", r.ID(), at),
		debit(ChargebackLoss, r.MerchantID(), r.Net(), "chargeback", r.ID(), at),
		debit(RevenueFees, r.MerchantID(), r.Fee(), "chargeback", r.ID(), at),
	}
}

func Balanced(entries []Entry) bool {
	var d, c shared.Money
	for _, e := range entries {
		d += e.Debit
		c += e.Credit
	}
	return d == c
}

type Repository interface {
	Append(ctx context.Context, entries []Entry) error
	// Balance devolve créditos − débitos de uma conta (para um EC ou, com "" , geral).
	Balance(ctx context.Context, account Account, merchantID string) (shared.Money, error)
}
