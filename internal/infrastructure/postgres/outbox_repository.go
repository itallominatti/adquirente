package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/itallominatti/adquirente/internal/application"
	"github.com/itallominatti/adquirente/internal/domain/shared"
)

type OutboxRepository struct{ db DBTX }

func NewOutboxRepository(db DBTX) *OutboxRepository { return &OutboxRepository{db: db} }

var _ application.OutboxRepository = (*OutboxRepository)(nil)

func (r *OutboxRepository) Append(ctx context.Context, events ...shared.Event) error {
	for _, ev := range events {
		payload, err := json.Marshal(ev)
		if err != nil {
			return fmt.Errorf("serializar evento %s: %w", ev.EventType(), err)
		}
		_, err = r.db.ExecContext(ctx, `INSERT INTO outbox (event_id, aggregate_id, event_type, payload, created_at)
			VALUES ($1, $2, $3, $4, $5)`, ev.EventID(), ev.AggregateID(), ev.EventType(), payload, ev.OccurredAt())
		if err != nil {
			return fmt.Errorf("gravar na outbox: %w", err)
		}
	}
	return nil
}

type ProcessedEventsRepository struct{ db DBTX }

func NewProcessedEventsRepository(db DBTX) *ProcessedEventsRepository {
	return &ProcessedEventsRepository{db: db}
}

var _ application.ProcessedEventsRepository = (*ProcessedEventsRepository)(nil)

func (r *ProcessedEventsRepository) MarkIfNew(ctx context.Context, consumer, eventID string) (bool, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO processed_events (consumer, event_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, consumer, eventID)
	if err != nil {
		return false, fmt.Errorf("marcar evento processado: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}
