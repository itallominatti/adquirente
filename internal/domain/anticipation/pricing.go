package anticipation

import (
	"math"

	"github.com/itallominatti/adquirente/internal/domain/shared"
)

type Pricer interface {
	PresentValue(net shared.Money, days int, rateMonth shared.Bps) shared.Money
}

type CompoundPricer struct{}

func (CompoundPricer) PresentValue(net shared.Money, days int, rateMonth shared.Bps) shared.Money {
	if days <= 0 || rateMonth == 0 {
		return net
	}
	rate := float64(rateMonth) / 10000
	factor := math.Pow(1+rate, float64(days)/30)
	return shared.Money(math.Round(float64(net) / factor))
}
