package shared

import "fmt"

type Money int64

type Bps int

func (m Money) IsPositive() bool { return m > 0 }
func (m Money) IsZero() bool     { return m == 0 }

func (m Money) String() string {
	v := int64(m)
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	return fmt.Sprintf("%sR$ %d,%02d", sign, v/100, v%100)
}

func ApplyBps(gross Money, rate Bps) Money {
	return Money(int64(gross) * int64(rate) / 10000)
}

func Split(total Money, n int) []Money {
	if n <= 0 {
		return nil
	}
	base := int64(total) / int64(n)
	rest := int64(total) - base*int64(n)
	parts := make([]Money, n)
	for i := range parts {
		parts[i] = Money(base)
	}
	parts[0] += Money(rest)
	return parts
}

func Sum(values ...Money) Money {
	var total Money
	for _, v := range values {
		total += v
	}
	return total
}
