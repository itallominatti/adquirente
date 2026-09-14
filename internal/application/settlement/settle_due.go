package settlement

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/itallominatti/adquirente/internal/application"
	"github.com/itallominatti/adquirente/internal/domain/ledger"
	"github.com/itallominatti/adquirente/internal/domain/merchant"
	"github.com/itallominatti/adquirente/internal/domain/receivable"
	"github.com/itallominatti/adquirente/internal/domain/settlement"
	"github.com/itallominatti/adquirente/internal/domain/shared"
)

type DailyLock interface {
	Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error)
	Release(ctx context.Context, key string) error
}

var ErrAlreadyRunning = errors.New("liquidação do dia já está em execução")

type Report struct {
	Date        time.Time
	Skipped     string // motivo, se não rodou (fim de semana, feriado)
	Merchants   int
	Settlements int
	Failed      int
	Paid        shared.Money
	Retained    shared.Money
}

type SettleDue struct {
	uow       application.UnitOfWork
	merchants merchant.Repository
	registry  receivable.Registry
	gateway   settlement.PaymentGateway
	calendar  shared.BusinessCalendar
	clock     shared.Clock
	lock      DailyLock
	batchSize int
	log       *slog.Logger
}

func NewSettleDue(uow application.UnitOfWork, merchants merchant.Repository, registry receivable.Registry, gateway settlement.PaymentGateway,
	calendar shared.BusinessCalendar, clock shared.Clock, lock DailyLock, batchSize int, log *slog.Logger) *SettleDue {
	return &SettleDue{uow: uow, merchants: merchants, registry: registry, gateway: gateway, calendar: calendar, clock: clock, lock: lock, batchSize: batchSize, log: log}
}

func (uc *SettleDue) Execute(ctx context.Context, day time.Time) (Report, error) {
	day = shared.DateOnly(day)
	rep := Report{Date: day}
	if !uc.calendar.IsBusinessDay(day) {
		rep.Skipped = "não é dia útil"
		return rep, nil
	}
	key := "settlement:" + day.Format("2006-01-02")
	ok, err := uc.lock.Acquire(ctx, key, 2*time.Hour)
	if err != nil {
		return rep, err
	}
	if !ok {
		return rep, ErrAlreadyRunning
	}
	defer func() { _ = uc.lock.Release(context.WithoutCancel(ctx), key) }()

	for {
		processed, err := uc.batch(ctx, day, &rep)
		if err != nil {
			return rep, err
		}
		if processed == 0 {
			return rep, nil
		}
	}
}

func (uc *SettleDue) batch(ctx context.Context, day time.Time, rep *Report) (int, error) {
	var count int
	err := uc.uow.Do(ctx, func(repos application.Repositories) error {
		list, err := repos.Receivables.ListDueOnForUpdate(ctx, day, uc.batchSize)
		if err != nil {
			return err
		}
		count = len(list)
		if count == 0 {
			return nil
		}
		byMerchant := map[string][]*receivable.Receivable{}
		for _, r := range list {
			byMerchant[r.MerchantID()] = append(byMerchant[r.MerchantID()], r)
		}
		now := uc.clock.Now()
		for merchantID, rs := range byMerchant {
			if err := uc.settleMerchant(ctx, repos, merchantID, rs, day, now, rep); err != nil {
				return err
			}
		}
		return nil
	})
	return count, err
}

func (uc *SettleDue) settleMerchant(ctx context.Context, repos application.Repositories, merchantID string,
	rs []*receivable.Receivable, day, now time.Time, rep *Report) error {
	rep.Merchants++
	m, err := uc.merchants.FindByID(ctx, merchantID)
	if err != nil {
		return fmt.Errorf("EC %s: %w", merchantID, err)
	}
	liens, err := uc.registry.CheckLiens(ctx, merchantID, day) // compliance: nunca pagar sem checar ônus
	if err != nil {
		return fmt.Errorf("consultar ônus de %s: %w", merchantID, err)
	}
	s, err := settlement.Build(day, m, rs, liens, now)
	if err != nil {
		return err
	}
	if err := repos.Settlements.Create(ctx, s); err != nil {
		return err
	}

	ref, sendErr := uc.gateway.Send(ctx, s)
	if sendErr != nil {
		rep.Failed++
		uc.log.ErrorContext(ctx, "banco rejeitou lote", "settlement_id", s.ID(), "merchant_id", merchantID, "err", sendErr)
		if err := s.MarkFailed(sendErr.Error(), now); err != nil {
			return err
		}
		if err := repos.Settlements.Update(ctx, s); err != nil {
			return err
		}
		return repos.Outbox.Append(ctx, s.PullEvents()...)
	}

	if err := s.MarkSent(ref, now); err != nil {
		return err
	}
	if err := s.MarkConfirmed(now); err != nil { // o banco simulado confirma na hora; o real confirmaria por arquivo de retorno
		return err
	}
	for _, r := range rs {
		if err := r.Settle(s.ID(), now); err != nil {
			return err
		}
	}
	if err := repos.Receivables.UpdateAll(ctx, rs); err != nil {
		return err
	}
	if err := repos.Settlements.Update(ctx, s); err != nil {
		return err
	}
	if entries := ledger.ForSettlement(s, now); len(entries) > 0 {
		if err := repos.Ledger.Append(ctx, entries); err != nil {
			return err
		}
	}
	rep.Settlements++
	rep.Paid += s.ToMerchant() + s.ToCreditors()
	rep.Retained += s.Retained()
	return repos.Outbox.Append(ctx, s.PullEvents()...)
}
