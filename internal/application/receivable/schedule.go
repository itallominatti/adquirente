package receivable

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/itallominatti/adquirente/internal/application"
	"github.com/itallominatti/adquirente/internal/domain/ledger"
	"github.com/itallominatti/adquirente/internal/domain/merchant"
	"github.com/itallominatti/adquirente/internal/domain/receivable"
	"github.com/itallominatti/adquirente/internal/domain/shared"
	"github.com/itallominatti/adquirente/internal/domain/transaction"
)

const consumerName = "scheduler"

type Scheduler struct {
	uow       application.UnitOfWork
	merchants merchant.Repository
	registry  receivable.Registry
	calendar  shared.BusinessCalendar
	clock     shared.Clock
	log       *slog.Logger
}

func NewScheduler(uow application.UnitOfWork, merchants merchant.Repository, registry receivable.Registry,
	calendar shared.BusinessCalendar, clock shared.Clock, log *slog.Logger) *Scheduler {
	return &Scheduler{uow: uow, merchants: merchants, registry: registry, calendar: calendar, clock: clock, log: log}
}

func (s *Scheduler) Handle(ctx context.Context, eventType string, payload []byte) error {
	switch eventType {
	case "transaction.captured":
		var ev transaction.TransactionCaptured
		if err := json.Unmarshal(payload, &ev); err != nil {
			return application.Permanent(fmt.Errorf("payload inválido: %w", err))
		}
		return s.onCaptured(ctx, ev)
	case "transaction.canceled":
		var ev transaction.TransactionCanceled
		if err := json.Unmarshal(payload, &ev); err != nil {
			return application.Permanent(fmt.Errorf("payload inválido: %w", err))
		}
		return s.onCanceled(ctx, ev)
	}
	return nil // tipos que não nos interessam são ignorados
}

func (s *Scheduler) onCaptured(ctx context.Context, ev transaction.TransactionCaptured) error {
	var created []*receivable.Receivable
	err := s.uow.Do(ctx, func(repos application.Repositories) error {
		isNew, err := repos.Processed.MarkIfNew(ctx, consumerName, ev.EventID())
		if err != nil {
			return err
		}
		if !isNew {
			s.log.InfoContext(ctx, "evento duplicado ignorado", "event_id", ev.EventID())
			return nil
		}
		tx, err := repos.Transactions.FindByID(ctx, ev.MerchantID, ev.AggregateID())
		if err != nil {
			return err
		}
		m, err := s.merchants.FindByID(ctx, ev.MerchantID)
		if err != nil {
			return err
		}
		now := s.clock.Now()
		list, err := receivable.Schedule(tx, m.FeePlan(), s.calendar, now)
		if err != nil {
			return application.Permanent(err) // transação não capturada: regra violada, não é transitório
		}
		if err := repos.Receivables.SaveAll(ctx, list); err != nil {
			return err
		}
		if err := repos.Ledger.Append(ctx, ledger.ForScheduled(list, now)); err != nil {
			return err
		}
		created = list
		return repos.Outbox.Append(ctx, receivable.NewReceivablesScheduled(list, now))
	})
	if err != nil || len(created) == 0 {
		return err
	}
	// Registro na registradora fora da transação de banco: se falhar, o Kafka reentrega
	// o evento; MarkIfNew impede duplicar a agenda, mas o registro precisa ser idempotente também.
	if err := s.registry.Register(ctx, receivable.GroupUnits(created)); err != nil {
		s.log.WarnContext(ctx, "registro de URs falhou; será refeito pelo job de reconciliação", "err", err)
	}
	return nil
}

func (s *Scheduler) onCanceled(ctx context.Context, ev transaction.TransactionCanceled) error {
	return s.uow.Do(ctx, func(repos application.Repositories) error {
		isNew, err := repos.Processed.MarkIfNew(ctx, consumerName, ev.EventID())
		if err != nil || !isNew {
			return err
		}
		list, err := repos.Receivables.ListByTransaction(ctx, ev.AggregateID())
		if err != nil {
			return err
		}
		now := s.clock.Now()
		var changed []*receivable.Receivable
		var entries []ledger.Entry
		for _, r := range list {
			if r.Status() != receivable.Scheduled {
				continue
			}
			if err := r.Cancel(now); err != nil {
				return err
			}
			changed = append(changed, r)
			entries = append(entries, ledger.ForChargeback(r, now)...) // mesmos lançamentos inversos da captura
		}
		if len(changed) == 0 {
			return nil
		}
		if err := repos.Receivables.UpdateAll(ctx, changed); err != nil {
			return err
		}
		return repos.Ledger.Append(ctx, entries)
	})
}
