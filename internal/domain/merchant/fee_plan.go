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

func (p FeePlan) MDR(product shared.Product, installments int) (shared.Bps, error) {
	switch {
	case product == shared.Debit && installments == 1:
		return p.debit, nil
	case product == shared.Credit && installments == 1:
		return p.creditOneShot, nil
	case product == shared.Credit && installments > 1:
		bps, ok := p.creditInstallments[installments]
		if !ok {
			return 0, ErrInstallmentsNotAllowed
		}
		return bps, nil
	}
	return 0, ErrInstallmentsNotAllowed
}

func (p FeePlan) AnticipationRateMonth() shared.Bps { return p.anticipationRateMonth }
func (p FeePlan) Debit() shared.Bps                 { return p.debit }
func (p FeePlan) CreditOneShot() shared.Bps         { return p.creditOneShot }

func (p FeePlan) CreditInstallments() map[int]shared.Bps {
	out := make(map[int]shared.Bps, len(p.creditInstallments))
	for k, v := range p.creditInstallments {
		out[k] = v
	}
	return out
}
