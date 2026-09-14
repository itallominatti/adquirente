package shared

import "time"

type BusinessCalendar interface {
	IsBusinessDay(t time.Time) bool
	NextBusinessDay(t time.Time) time.Time
	AddBusinessDays(t time.Time, n int) time.Time
}

type BrazilCalendar struct{}

func (BrazilCalendar) IsBusinessDay(t time.Time) bool {
	if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		return false
	}
	return !isNationalHoliday(t)
}

func (c BrazilCalendar) NextBusinessDay(t time.Time) time.Time {
	d := DateOnly(t)
	for !c.IsBusinessDay(d) {
		d = d.AddDate(0, 0, 1)
	}
	return d
}

func (c BrazilCalendar) AddBusinessDays(t time.Time, n int) time.Time {
	d := DateOnly(t)
	for n > 0 {
		d = d.AddDate(0, 0, 1)
		if c.IsBusinessDay(d) {
			n--
		}
	}
	return d
}

func isNationalHoliday(t time.Time) bool {
	m, d := t.Month(), t.Day()
	switch {
	case m == time.January && d == 1, // Confraternização Universal
		m == time.April && d == 21,    // Tiradentes
		m == time.May && d == 1,       // Dia do Trabalho
		m == time.September && d == 7, // Independência
		m == time.October && d == 12,  // Nossa Senhora Aparecida
		m == time.November && d == 2,  // Finados
		m == time.November && d == 15, // Proclamação da República
		m == time.November && d == 20, // Consciência Negra (nacional desde 2024)
		m == time.December && d == 25: // Natal
		return true
	}
	easter := easterSunday(t.Year())
	day := DateOnly(t)
	// segunda e terça de Carnaval, Sexta-feira Santa, Corpus Christi
	for _, offset := range []int{-48, -47, -2, 60} {
		if day.Equal(easter.AddDate(0, 0, offset)) {
			return true
		}
	}
	return false
}

// easterSunday calcula o domingo de Páscoa pelo algoritmo de Meeus/Jones/Butcher.
func easterSunday(year int) time.Time {
	a := year % 19
	b := year / 100
	c := year % 100
	d := b / 4
	e := b % 4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i := c / 4
	k := c % 4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	month := (h + l - 7*m + 114) / 31
	day := ((h + l - 7*m + 114) % 31) + 1
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}
