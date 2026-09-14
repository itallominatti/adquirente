package shared

import "errors"

type Product string

const (
	Debit  Product = "DEBIT"
	Credit Product = "CREDIT"
)

var ErrInvalidProduct = errors.New("Produto inválido: use DEBIT ou CREDIT")

func ParseProduct(s string) (Product, error) {
	switch Product(s) {
	case Debit, Credit:
		return Product(s), nil
	}
	return "", ErrInvalidProduct
}
