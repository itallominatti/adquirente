package kafka

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

type Relay struct {
	db       *sql.DB
	pub      *Publisher
	log      *slog.Logger
	interval time.Duration
	batch    int
}

func NewRelay(db *sql.DB, pub *Publisher, log *slog.Logger, interval time.Duration, batch int) *Relay {
	return &Relay{db: db, pub: pub, log: log, interval: interval, batch: batch}
}

func (r *Relay) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		n, err := r.publishBatch(ctx)
		if err != nil {
			r.log.ErrorContext(ctx, "relay da outbox falhou", "err", err)
		}
		if n == r.batch { // ainda há fila: não espera o ticker
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *Relay) publishBatch(ctx context.Context) (int, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `SELECT id, event_id, aggregate_id, event_type, payload FROM outbox
		WHERE published_at IS NULL ORDER BY id LIMIT $1 FOR UPDATE SKIP LOCKED`, r.batch)
	if err != nil {
		return 0, err
	}
	type row struct {
		id                              int64
		eventID, aggregateID, eventType string
		payload                         []byte
	}
	var pending []row
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.id, &x.eventID, &x.aggregateID, &x.eventType, &x.payload); err != nil {
			rows.Close()
			return 0, err
		}
		pending = append(pending, x)
	}
	rows.Close()
	if len(pending) == 0 {
		return 0, nil
	}

	var published []int64
	for _, x := range pending {
		// tópico = tipo do evento ("transaction.captured"); chave = id do agregado (ordem por transação/EC)
		err := r.pub.Publish(ctx, x.eventType, x.aggregateID, x.payload, map[string]string{
			"event_id": x.eventID, "event_type": x.eventType,
		})
		if err != nil {
			r.log.WarnContext(ctx, "falha ao publicar; ficará para a próxima rodada", "event_id", x.eventID, "err", err)
			break // mantém a ordem: não pula eventos
		}
		published = append(published, x.id)
	}
	if len(published) > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE outbox SET published_at = now() WHERE id = ANY($1)`, published); err != nil {
			return 0, fmt.Errorf("marcar publicados: %w", err)
		}
	}
	return len(published), tx.Commit()
}
