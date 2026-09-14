package merchant

import "errors"

type BankAccount struct {
	BankCode string
	Branch   string
	Number   string
	Kind     AccountKind
}

type AccountKind string

const (
	Checking AccountKind = "CHECKING" // conta corrente
	Savings  AccountKind = "SAVINGS"  // poupança
	Payment  AccountKind = "PAYMENT"  // conta de pagamento (fintechs)
)

var ErrInvalidBankAccount = errors.New("dados bancários inválidos")

func NewBankAccount(bankCode, branch, number string, kind AccountKind) (BankAccount, error) {
	if len(bankCode) != 3 || branch == "" || number == "" {
		return BankAccount{}, ErrInvalidBankAccount
	}
	switch kind {
	case Checking, Savings, Payment:
	default:
		return BankAccount{}, ErrInvalidBankAccount
	}
	return BankAccount{BankCode: bankCode, Branch: branch, Number: number, Kind: kind}, nil
}

func (b BankAccount) IsZero() bool { return b.BankCode == "" }
