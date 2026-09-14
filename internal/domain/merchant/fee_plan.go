package merchant

import (
	"errors"

	"github.com/itallominatti/adquirente/internal/domain/shared"
)

type FeePlan struct {
	debit                 shared.Bps
	creditOneShot         shared.Bps
	creditInstallments    map[int]shared.Bps // parcelas (2..12) → MDR
	anticipationRateMonth shared.Bps
}

var (
	ErrInstallmentsNotAllowed = errors.New("número de parcelas não permitido pelo plano de taxas")
	ErrInvalidFeePlan         = errors.New("plano de taxas inválido")
)

func NewFeePlan(debit, creditOneShot shared.Bps, creditInstallments map[int]shared.Bps, anticipationRateMonth shared.Bps) (FeePlan, error) {
	if debit < 0 || creditOneShot < 0 || anticipationRateMonth < 0 || debit > 10000 || creditOneShot > 10000 {
		return FeePlan{}, ErrInvalidFeePlan
	}
	rates := make(map[int]shared.Bps, len(creditInstallments))
	for n, bps := range creditInstallments {
		if n < 2 || n > 12 || bps < 0 || bps > 10000 {
			return FeePlan{}, ErrInvalidFeePlan
		}
		rates[n] = bps
	}
	return FeePlan{debit: debit, creditOneShot: creditOneShot, creditInstallments: rates, anticipationRateMonth: anticipationRateMonth}, nil
}
