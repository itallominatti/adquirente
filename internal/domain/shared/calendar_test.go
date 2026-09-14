package shared

import (
	"testing"
	"time"
)

func date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func TestBrazilCalendar(t *testing.T) {
	cal := BrazilCalendar{}

	if cal.IsBusinessDay(date(2026, time.September, 12)) { // sábado
		t.Fatal("sábado não é dia útil")
	}
	if cal.IsBusinessDay(date(2026, time.September, 7)) { // Independência
		t.Fatal("7 de setembro não é dia útil")
	}
	if !cal.IsBusinessDay(date(2026, time.September, 14)) { // segunda
		t.Fatal("segunda comum é dia útil")
	}

	// Páscoa 2026 = 05/04; Carnaval (ter.) = 17/02; Sexta Santa = 03/04; Corpus Christi = 04/06
	if got := easterSunday(2026); !got.Equal(date(2026, time.April, 5)) {
		t.Fatalf("Páscoa 2026 = %v", got)
	}
	for _, h := range []time.Time{date(2026, time.February, 17), date(2026, time.April, 3), date(2026, time.June, 4)} {
		if cal.IsBusinessDay(h) {
			t.Fatalf("%v é feriado móvel", h)
		}
	}

	// quinta comum fica como está
	if got := cal.NextBusinessDay(date(2026, time.April, 9)); !got.Equal(date(2026, time.April, 9)) {
		t.Fatalf("NextBusinessDay(quinta) = %v", got)
	}
	// sábado 12/09 -> segunda 14/09
	if got := cal.NextBusinessDay(date(2026, time.September, 12)); !got.Equal(date(2026, time.September, 14)) {
		t.Fatalf("NextBusinessDay(sábado) = %v", got)
	}
	// D+1 útil de sexta 04/09/2026 pula sábado, domingo e o feriado de segunda (07/09) -> terça 08/09
	if got := cal.AddBusinessDays(date(2026, time.September, 4), 1); !got.Equal(date(2026, time.September, 8)) {
		t.Fatalf("AddBusinessDays(sexta antes do 7/9, 1) = %v", got)
	}
}
