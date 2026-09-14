package receivable

import (
	"context"
	"time"

	"github.com/itallominatti/adquirente/internal/domain/receivable"
)

type Query struct{ receivables receivable.Repository }

func NewQuery(receivables receivable.Repository) *Query { return &Query{receivables: receivables} }

func (q *Query) List(ctx context.Context, merchantID string, f receivable.ListFilter) ([]*receivable.Receivable, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	return q.receivables.ListByMerchant(ctx, merchantID, f)
}

func (q *Query) Summary(ctx context.Context, merchantID string, from, to time.Time) ([]receivable.DailySummary, error) {
	return q.receivables.Summary(ctx, merchantID, from, to)
}
