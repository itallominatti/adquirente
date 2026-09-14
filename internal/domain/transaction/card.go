package transaction

import (
	"errors"
	"log/slog"
	"strings"
)

type CardInfo struct {
	Token string
	Brand string
	BIN   string // 6 primeiros dígitos
	Last4 string
}

var (
	ErrInvalidPAN   = errors.New("número de cartão inválido")
	ErrInvalidToken = errors.New("token de cartão inválido")
)

func CardInfoFromPAN(token, pan string) (CardInfo, error) {
	if token == "" {
		return CardInfo{}, ErrInvalidToken
	}
	if !Luhn(pan) {
		return CardInfo{}, ErrInvalidPAN
	}
	return CardInfo{Token: token, Brand: brandOf(pan), BIN: pan[:6], Last4: pan[len(pan)-4:]}, nil
}

func Luhn(pan string) bool {
	if len(pan) < 13 || len(pan) > 19 {
		return false
	}
	sum := 0
	double := false
	for i := len(pan) - 1; i >= 0; i-- {
		c := pan[i]
		if c < '0' || c > '9' {
			return false
		}
		d := int(c - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}

func brandOf(pan string) string {
	switch {
	case strings.HasPrefix(pan, "4"):
		return "VISA"
	case pan[0] == '5' && pan[1] >= '1' && pan[1] <= '5', strings.HasPrefix(pan, "2"):
		return "MASTERCARD"
	case strings.HasPrefix(pan, "6362"), strings.HasPrefix(pan, "5067"), strings.HasPrefix(pan, "4576"):
		return "ELO"
	case strings.HasPrefix(pan, "34"), strings.HasPrefix(pan, "37"):
		return "AMEX"
	}
	return "UNKNOWN"
}

func MaskPAN(pan string) string {
	if len(pan) < 10 {
		return strings.Repeat("*", len(pan))
	}
	return pan[:6] + strings.Repeat("*", len(pan)-10) + pan[len(pan)-4:]
}

type PAN string

func (p PAN) LogValue() slog.Value { return slog.StringValue(MaskPAN(string(p))) }
func (p PAN) String() string       { return MaskPAN(string(p)) }
