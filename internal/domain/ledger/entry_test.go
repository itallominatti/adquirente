package ledger

import (
	"testing"
	"time"

	"github.com/itallominatti/adquirente/internal/domain/anticipation"
	"github.com/itallominatti/adquirente/internal/domain/receivable"
	"github.com/itallominatti/adquirente/internal/domain/shared"
)

func TestEntriesAlwaysBalance(t *testing.T) {
	at := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	due := at.AddDate(0, 0, 30)
	r1 := receivable.Restore("rcv_1", "tx_1", "m_1", 1, 2, shared.Credit, "VISA", 50000, 1750, 48250, due, receivable.Scheduled, "", "", 1, at, at)
	r2 := receivable.Restore("rcv_2", "tx_1", "m_1", 2, 2, shared.Credit, "VISA", 50000, 1750, 48250, due.AddDate(0, 0, 30), receivable.Scheduled, "", "", 1, at, at)

	sched := ForScheduled([]*receivable.Receivable{r1, r2}, at)
	if len(sched) != 6 || !Balanced(sched) {
		t.Fatalf("scheduled: %d lançamentos, balanced=%v", len(sched), Balanced(sched))
	}

	a, err := anticipation.Simulate("m_1", []*receivable.Receivable{r1, r2}, at, 200, anticipation.CompoundPricer{})
	if err != nil {
		t.Fatal(err)
	}
	ant := ForAnticipation(a, at)
	if !Balanced(ant) {
		t.Fatal("anticipation não fecha")
	}

	cb := ForChargeback(r1, at)
	if !Balanced(cb) {
		t.Fatal("chargeback não fecha")
	}

	// um lançamento solto não fecha
	if Balanced([]Entry{{Debit: 1}}) {
		t.Fatal("deveria detectar desbalanceamento")
	}
}
