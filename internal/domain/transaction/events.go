package transaction

import (
	"time"

	"github.com/itallominatti/adquirente/internal/domain/shared"
)

type TransactionAuthorized struct {
	shared.BaseEvent
	MerchantID        string         `json:"merchant_id"`
	Amount            shared.Money   `json:"amount"`
	Product           shared.Product `json:"product"`
	Installments      int            `json:"installments"`
	AuthorizationCode string         `json:"authorization_code"`
	NSU               string         `json:"nsu"`
	CardBrand         string         `json:"card_brand"`
	CardLast4         string         `json:"card_last4"`
}

type TransactionDenied struct {
	shared.BaseEvent
	MerchantID   string       `json:"merchant_id"`
	Amount       shared.Money `json:"amount"`
	ResponseCode string       `json:"response_code"`
}

type TransactionCaptured struct {
	shared.BaseEvent
	MerchantID   string         `json:"merchant_id"`
	Amount       shared.Money   `json:"amount"`
	Product      shared.Product `json:"product"`
	Installments int            `json:"installments"`
	CardBrand    string         `json:"card_brand"`
	CapturedAt   time.Time      `json:"captured_at"`
}

type TransactionCanceled struct {
	shared.BaseEvent
	MerchantID string       `json:"merchant_id"`
	Amount     shared.Money `json:"amount"`
}

type TransactionChargebacked struct {
	shared.BaseEvent
	MerchantID string       `json:"merchant_id"`
	Amount     shared.Money `json:"amount"`
	ReasonCode string       `json:"reason_code"`
}
