package merchant

import (
	"context"
	"fmt"

	"github.com/itallominatti/adquirente/internal/domain/merchant"
	"github.com/itallominatti/adquirente/internal/domain/shared"
)

type OnboardCommand struct {
	Document             string
	LegalName            string
	MCC                  string
	BankCode             string
	Branch               string
	AccountNumber        string
	AccountKind          string
	DebitBps             int
	CreditBps            int
	CreditInstallmentBps map[int]int // parcelas → bps
	AnticipationBpsMonth int
	WebhookURL           string
}

type Onboard struct {
	merchants merchant.Repository
	clock     shared.Clock
}

func NewOnboard(merchants merchant.Repository, clock shared.Clock) *Onboard {
	return &Onboard{merchants: merchants, clock: clock}
}

func (uc *Onboard) Execute(ctx context.Context, cmd OnboardCommand) (*merchant.Merchant, error) {
	doc, err := merchant.NewDocument(cmd.Document)
	if err != nil {
		return nil, err
	}
	bank, err := merchant.NewBankAccount(cmd.BankCode, cmd.Branch, cmd.AccountNumber, merchant.AccountKind(cmd.AccountKind))
	if err != nil {
		return nil, err
	}
	installments := make(map[int]shared.Bps, len(cmd.CreditInstallmentBps))
	for n, bps := range cmd.CreditInstallmentBps {
		installments[n] = shared.Bps(bps)
	}
	plan, err := merchant.NewFeePlan(shared.Bps(cmd.DebitBps), shared.Bps(cmd.CreditBps), installments, shared.Bps(cmd.AnticipationBpsMonth))
	if err != nil {
		return nil, err
	}
	now := uc.clock.Now()
	m, err := merchant.New(doc, cmd.LegalName, cmd.MCC, bank, plan, now)
	if err != nil {
		return nil, err
	}
	if cmd.WebhookURL != "" {
		if err := m.SetWebhook(cmd.WebhookURL, now); err != nil {
			return nil, err
		}
	}
	if err := uc.merchants.Create(ctx, m); err != nil {
		return nil, fmt.Errorf("salvar estabelecimento: %w", err)
	}
	return m, nil
}
