package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/itallominatti/adquirente/internal/domain/merchant"
	"github.com/itallominatti/adquirente/internal/domain/receivable"
	"github.com/itallominatti/adquirente/internal/domain/shared"
)

type Fake struct {
	mu     sync.Mutex
	liens  []receivable.Lien
	units  []receivable.Unit
	owners map[string]string // receivable_id → dono
}

var _ receivable.Registry = (*Fake)(nil)

func NewFake(liensFile string) (*Fake, error) {
	f := &Fake{owners: map[string]string{}}
	if liensFile == "" {
		return f, nil
	}
	raw, err := os.ReadFile(liensFile)
	if err != nil {
		return nil, fmt.Errorf("ler arquivo de ônus: %w", err)
	}
	var rows []struct {
		MerchantID   string `json:"merchant_id"`
		DueDate      string `json:"due_date"`
		Amount       int64  `json:"amount"`
		CreditorName string `json:"creditor_name"`
		BankCode     string `json:"bank_code"`
		Branch       string `json:"branch"`
		Number       string `json:"number"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("arquivo de ônus inválido: %w", err)
	}
	for _, r := range rows {
		due, err := time.Parse("2006-01-02", r.DueDate)
		if err != nil {
			return nil, fmt.Errorf("data inválida %q: %w", r.DueDate, err)
		}
		acc, err := merchant.NewBankAccount(r.BankCode, r.Branch, r.Number, merchant.Checking)
		if err != nil {
			return nil, err
		}
		f.liens = append(f.liens, receivable.Lien{MerchantID: r.MerchantID, DueDate: shared.DateOnly(due), Amount: shared.Money(r.Amount), CreditorName: r.CreditorName, Creditor: acc})
	}
	return f, nil
}

func (f *Fake) Register(_ context.Context, units []receivable.Unit) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.units = append(f.units, units...)
	return nil
}

func (f *Fake) CheckLiens(_ context.Context, merchantID string, dueDate time.Time) ([]receivable.Lien, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []receivable.Lien
	for _, l := range f.liens {
		if l.MerchantID == merchantID && l.DueDate.Equal(shared.DateOnly(dueDate)) {
			out = append(out, l)
		}
	}
	return out, nil
}

func (f *Fake) TransferOwnership(_ context.Context, receivableIDs []string, newOwner string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range receivableIDs {
		f.owners[id] = newOwner
	}
	return nil
}
