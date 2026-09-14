package anticipation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/itallominatti/adquirente/internal/application"
	"github.com/itallominatti/adquirente/internal/domain/anticipation"
	"github.com/itallominatti/adquirente/internal/domain/ledger"
	"github.com/itallominatti/adquirente/internal/domain/merchant"
	"github.com/itallominatti/adquirente/internal/domain/receivable"
	"github.com/itallominatti/adquirente/internal/domain/shared"
)

var ErrReceivableLiened = errors.New("há ônus registrado sobre recebíveis desta data; antecipação não permitida")

type Anticipate struct {
	uow         application.UnitOfWork
	receivables receivable.Repository // leitura sem transação, para simular
	merchants   merchant.Repository
	registry    receivable.Registry
	pricer      anticipation.Pricer
	clock       shared.Clock
	log         *slog.Logger
}

func NewAnticipate(uow application.UnitOfWork, receivables receivable.Repository, merchants merchant.Repository,
	registry receivable.Registry, pricer anticipation.Pricer, clock shared.Clock, log *slog.Logger) *Anticipate {
	return &Anticipate{uow: uow, receivables: receivables, merchants: merchants, registry: registry, pricer: pricer, clock: clock, log: log}
}

func (uc *Anticipate) Simulate(ctx context.Context, merchantID string, receivableIDs []string) (*anticipation.Anticipation, error) {
	m, err := uc.merchants.FindByID(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	rs, err := uc.receivables.FindByIDs(ctx, merchantID, receivableIDs)
	if err != nil {
		return nil, err
	}
	if len(rs) != len(receivableIDs) {
		return nil, fmt.Errorf("%w: algum recebível não existe ou não é seu", shared.ErrNotFound)
	}
	return anticipation.Simulate(merchantID, rs, uc.clock.Now(), m.FeePlan().AnticipationRateMonth(), uc.pricer)
}

func (uc *Anticipate) Request(ctx context.Context, merchantID string, receivableIDs []string) (*anticipation.Anticipation, error) {
	m, err := uc.merchants.FindByID(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	var a *anticipation.Anticipation
	err = uc.uow.Do(ctx, func(repos application.Repositories) error {
		rs, err := repos.Receivables.FindForUpdate(ctx, merchantID, receivableIDs)
		if err != nil {
			return err
		}
		if len(rs) != len(receivableIDs) {
			return fmt.Errorf("%w: algum recebível não existe ou não é seu", shared.ErrNotFound)
		}
		now := uc.clock.Now()
		a, err = anticipation.Simulate(merchantID, rs, now, m.FeePlan().AnticipationRateMonth(), uc.pricer)
		if err != nil {
			return err
		}
		if err := uc.ensureNoLiens(ctx, merchantID, rs); err != nil {
			return err
		}
		if err := a.Confirm(rs, now); err != nil {
			return err
		}
		if err := repos.Anticipations.Create(ctx, a); err != nil {
			return err
		}
		if err := repos.Receivables.UpdateAll(ctx, rs); err != nil {
			return err
		}
		if err := repos.Ledger.Append(ctx, ledger.ForAnticipation(a, now)); err != nil {
			return err
		}
		return repos.Outbox.Append(ctx, a.PullEvents()...)
	})
	if err != nil {
		return nil, err
	}
	// A troca de titularidade na registradora acontece após o commit; se falhar, o job
	// de reconciliação refaz a partir do evento receivable.anticipated.
	if err := uc.registry.TransferOwnership(ctx, a.ReceivableIDs(), "ADQUIRENTE"); err != nil {
		uc.log.WarnContext(ctx, "troca de titularidade falhou; reconciliação refará", "anticipation_id", a.ID(), "err", err)
	}
	return a, nil
}

func (uc *Anticipate) ensureNoLiens(ctx context.Context, merchantID string, rs []*receivable.Receivable) error {
	seen := map[time.Time]bool{}
	for _, r := range rs {
		if seen[r.DueDate()] {
			continue
		}
		seen[r.DueDate()] = true
		liens, err := uc.registry.CheckLiens(ctx, merchantID, r.DueDate())
		if err != nil {
			return fmt.Errorf("consultar ônus: %w", err)
		}
		if len(liens) > 0 {
			return fmt.Errorf("%w (%s)", ErrReceivableLiened, r.DueDate().Format("2006-01-02"))
		}
	}
	return nil
}

func (uc *Anticipate) Get(ctx context.Context, merchantID, id string) (*anticipation.Anticipation, error) {
	var a *anticipation.Anticipation
	err := uc.uow.Do(ctx, func(repos application.Repositories) error {
		var err error
		a, err = repos.Anticipations.FindByID(ctx, merchantID, id)
		return err
	})
	return a, err
}
