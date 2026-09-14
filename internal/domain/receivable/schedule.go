package receivable

import (
	"errors"
	"time"

	"github.com/itallominatti/adquirente/internal/domain/merchant"
	"github.com/itallominatti/adquirente/internal/domain/shared"
	"github.com/itallominatti/adquirente/internal/domain/transaction"
)

var ErrNotCaptured = errors.New("só transações capturadas geram recebíveis")

func Schedule(tx *transaction.Transaction, plan merchant.FeePlan, cal shared.BusinessCalendar, now time.Time) ([]*Receivable, error) {
	if tx.Status() != transaction.Captured || tx.CapturedAt() == nil {
		return nil, ErrNotCaptured
	}
	mdr, err := plan.MDR(tx.Product(), tx.Installments())
	if err != nil {
		return nil, err
	}
	n := tx.Installments()
	fee := shared.ApplyBps(tx.Amount(), mdr)
	grossParts := shared.Split(tx.Amount(), n)
	feeParts := shared.Split(fee, n)
	captured := shared.DateOnly(*tx.CapturedAt())

	out := make([]*Receivable, 0, n)
	for k := 1; k <= n; k++ {
		var due time.Time
		if tx.Product() == shared.Debit {
			due = cal.AddBusinessDays(captured, 1)
		} else {
			due = cal.NextBusinessDay(captured.AddDate(0, 0, 30*k))
		}
		out = append(out, newReceivable(tx.ID(), tx.MerchantID(), k, n, tx.Product(), tx.Card().Brand,
			grossParts[k-1], feeParts[k-1], due, now))
	}
	return out, nil
}

type Unit struct {
	MerchantID string
	Brand      string
	Product    shared.Product
	DueDate    time.Time
	Total      shared.Money
	Count      int
}

func GroupUnits(list []*Receivable) []Unit {
	type key struct {
		merchant string
		brand    string
		product  shared.Product
		due      time.Time
	}
	index := map[key]int{}
	var units []Unit
	for _, r := range list {
		k := key{r.merchantID, r.brand, r.product, r.dueDate}
		if i, ok := index[k]; ok {
			units[i].Total += r.net
			units[i].Count++
			continue
		}
		index[k] = len(units)
		units = append(units, Unit{MerchantID: r.merchantID, Brand: r.brand, Product: r.product, DueDate: r.dueDate, Total: r.net, Count: 1})
	}
	return units
}
