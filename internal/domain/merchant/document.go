package merchant

import (
	"errors"
	"strings"
)

type Document string

var ErrInvalidDocument = errors.New("CNPJ inválido")

func NewDocument(raw string) (Document, error) {
	clean := strings.ToUpper(strings.NewReplacer(".", "", "/", "", "-", "", " ", "").Replace(raw))
	if len(clean) != 14 {
		return "", ErrInvalidDocument
	}
	for i, ch := range clean {
		isDigit := ch >= '0' && ch <= '9'
		isLetter := ch >= 'A' && ch <= 'Z'
		if i >= 12 && !isDigit {
			return "", ErrInvalidDocument
		}
		if !isDigit && !isLetter {
			return "", ErrInvalidDocument
		}
	}
	if strings.Count(clean, string(clean[0])) == 14 {
		return "", ErrInvalidDocument
	}
	if checkDigit(clean[:12]) != int(clean[12]-'0') || checkDigit(clean[:13]) != int(clean[13]-'0') {
		return "", ErrInvalidDocument
	}
	return Document(clean), nil
}

func checkDigit(base string) int {
	weight := 2
	sum := 0
	for i := len(base) - 1; i >= 0; i-- {
		sum += int(base[i]-'0') * weight
		weight++
		if weight > 9 {
			weight = 2
		}
	}
	rest := sum % 11
	if rest < 2 {
		return 0
	}
	return 11 - rest
}

func (d Document) String() string { return string(d) }

func (d Document) Masked() string {
	s := string(d)
	if len(s) != 14 {
		return "**************"
	}
	return "**.***.***/" + s[8:12] + "-" + s[12:]
}
