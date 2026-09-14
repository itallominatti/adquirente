package transaction

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/itallominatti/adquirente/internal/application"
	"github.com/itallominatti/adquirente/internal/domain/merchant"
	"github.com/itallominatti/adquirente/internal/domain/shared"
	"github.com/itallominatti/adquirente/internal/domain/transaction"
)

type AuthorizeCommand struct {
	MerchantID   string
	Amount       int64
	Product      string
	Installments int
	CardToken    string
}

type Rule func(ctx context.Context, m *merchant.Merchant, tx *transaction.Transaction) error

var ErrIssuerUnavailable = errors.New("emissor indisponível")

type Authorize struct {
	merchants     merchant.Repository
	vault         transaction.CardVault
	issuer        transaction.IssuerGateway
	uow           application.UnitOfWork
	clock         shared.Clock
	rules         []Rule
	issuerTimeout time.Duration
}

func NewAuthorize(merchants merchant.Repository, vault transaction.CardVault, issuer transaction.IssuerGateway,
	uow application.UnitOfWork, clock shared.Clock, issuerTimeout time.Duration, rules ...Rule) *Authorize {
	return &Authorize{merchants: merchants, vault: vault, issuer: issuer, uow: uow, clock: clock, rules: rules, issuerTimeout: issuerTimeout}
}

func (uc *Authorize) Execute(ctx context.Context, cmd AuthorizeCommand) (*transaction.Transaction, error) {
	m, err := uc.merchants.FindByID(ctx, cmd.MerchantID)
	if err != nil {
		return nil, err
	}
	product, err := shared.ParseProduct(cmd.Product)
	if err != nil {
		return nil, err
	}

	card, err := uc.vault.Detokenize(ctx, cmd.CardToken)
	if err != nil {
		return nil, fmt.Errorf("vault: %w", err)
	}
	info, err := transaction.CardInfoFromPAN(cmd.CardToken, string(card.PAN))
	if err != nil {
		return nil, err
	}

	now := uc.clock.Now()
	tx, err := transaction.New(m, shared.Money(cmd.Amount), product, cmd.Installments, info, now)
	if err != nil {
		return nil, err
	}
	for _, rule := range uc.rules {
		if err := rule(ctx, m, tx); err != nil {
			return nil, err
		}
	}

	issuerCtx, cancel := context.WithTimeout(ctx, uc.issuerTimeout)
	resp, issuerErr := uc.issuer.Authorize(issuerCtx, transaction.AuthorizationRequest{
		TransactionID: tx.ID(), PAN: card.PAN, Expiry: card.Expiry, Amount: tx.Amount(),
		Product: tx.Product(), Installments: tx.Installments(), MerchantMCC: m.MCC(),
	})
	cancel()
	card = transaction.CardData{} // o PAN sai da memória assim que não é mais necessário

	switch {
	case errors.Is(issuerErr, context.DeadlineExceeded):
		// Saga: não sabemos se o emissor aprovou. Mandamos desfazer (reversal) e negamos.
		// context.WithoutCancel: o reversal precisa ir mesmo que o cliente HTTP já tenha desistido.
		_ = uc.issuer.Reverse(context.WithoutCancel(ctx), tx.ID())
		resp = transaction.AuthorizationResponse{Approved: false, ResponseCode: "91"}
	case issuerErr != nil:
		return nil, fmt.Errorf("%w: %v", ErrIssuerUnavailable, issuerErr)
	}

	err = uc.uow.Do(ctx, func(repos application.Repositories) error {
		if resp.Approved {
			nsu, err := repos.NSU.Next(ctx)
			if err != nil {
				return err
			}
			if err := tx.Authorize(resp.AuthorizationCode, nsu, uc.clock.Now()); err != nil {
				return err
			}
		} else if err := tx.Deny(resp.ResponseCode, uc.clock.Now()); err != nil {
			return err
		}
		if err := repos.Transactions.Create(ctx, tx); err != nil {
			return err
		}
		return repos.Outbox.Append(ctx, tx.PullEvents()...)
	})
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func MaxAmountRule(limit shared.Money) Rule {
	return func(_ context.Context, _ *merchant.Merchant, tx *transaction.Transaction) error {
		if tx.Amount() > limit {
			return fmt.Errorf("%w: acima do limite de %s por transação", ErrLimitExceeded, limit)
		}
		return nil
	}
}

var ErrLimitExceeded = errors.New("limite excedido")
