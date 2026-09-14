package http

import (
	"time"

	"github.com/itallominatti/adquirente/internal/domain/anticipation"
	"github.com/itallominatti/adquirente/internal/domain/chargeback"
	"github.com/itallominatti/adquirente/internal/domain/merchant"
	"github.com/itallominatti/adquirente/internal/domain/receivable"
	"github.com/itallominatti/adquirente/internal/domain/settlement"
	"github.com/itallominatti/adquirente/internal/domain/transaction"
)

// DTOs: structs "burras" só para transporte. A validação de FORMATO fica nas tags `binding`;
// a validação de NEGÓCIO fica no domínio. As respostas nunca expõem PAN.

// ---------- requests ----------

type CreateMerchantRequest struct {
	Document  string `json:"document" binding:"required"`
	LegalName string `json:"legal_name" binding:"required"`
	MCC       string `json:"mcc" binding:"required,len=4"`
	Bank      struct {
		Code   string `json:"code" binding:"required,len=3"`
		Branch string `json:"branch" binding:"required"`
		Number string `json:"number" binding:"required"`
		Kind   string `json:"kind" binding:"required,oneof=CHECKING SAVINGS PAYMENT"`
	} `json:"bank_account" binding:"required"`
	FeePlan struct {
		DebitBps             int         `json:"debit_bps" binding:"min=0,max=10000"`
		CreditBps            int         `json:"credit_bps" binding:"min=0,max=10000"`
		CreditInstallmentBps map[int]int `json:"credit_installments_bps"`
		AnticipationBpsMonth int         `json:"anticipation_bps_month" binding:"min=0,max=10000"`
	} `json:"fee_plan" binding:"required"`
	WebhookURL string `json:"webhook_url"`
}

type UpdateMerchantSettingsRequest struct {
	WebhookURL       *string `json:"webhook_url"`
	AutoAnticipation *bool   `json:"auto_anticipation"`
}

type TokenizeCardRequest struct {
	PAN    string `json:"pan" binding:"required,numeric,min=13,max=19"`
	Expiry string `json:"expiry" binding:"required,len=5"`
	// Sem campo CVV, de propósito: o CVV nunca chega ao nosso sistema.
}

type AuthorizeRequest struct {
	Amount       int64  `json:"amount" binding:"required,gt=0"`
	Product      string `json:"product" binding:"required,oneof=DEBIT CREDIT"`
	Installments int    `json:"installments" binding:"required,min=1,max=12"`
	CardToken    string `json:"card_token" binding:"required"`
}

type AnticipationRequest struct {
	ReceivableIDs []string `json:"receivable_ids" binding:"required,min=1,max=500"`
}

type OpenChargebackRequest struct {
	MerchantID    string `json:"merchant_id" binding:"required"`
	TransactionID string `json:"transaction_id" binding:"required"`
	ReasonCode    string `json:"reason_code" binding:"required"`
	Amount        int64  `json:"amount" binding:"required,gt=0"`
}

// ---------- responses ----------

type MerchantResponse struct {
	ID               string    `json:"id"`
	Document         string    `json:"document"` // mascarado
	LegalName        string    `json:"legal_name"`
	MCC              string    `json:"mcc"`
	Status           string    `json:"status"`
	WebhookURL       string    `json:"webhook_url,omitempty"`
	AutoAnticipation bool      `json:"auto_anticipation"`
	CreatedAt        time.Time `json:"created_at"`
}

func toMerchantResponse(m *merchant.Merchant) MerchantResponse {
	return MerchantResponse{ID: m.ID(), Document: m.Document().Masked(), LegalName: m.LegalName(), MCC: m.MCC(),
		Status: string(m.Status()), WebhookURL: m.WebhookURL(), AutoAnticipation: m.AutoAnticipation(), CreatedAt: m.CreatedAt()}
}

type CardTokenResponse struct {
	Token string `json:"token"`
	Brand string `json:"brand"`
	BIN   string `json:"bin"`
	Last4 string `json:"last4"`
}

type TransactionResponse struct {
	ID                string     `json:"id"`
	MerchantID        string     `json:"merchant_id"`
	Amount            int64      `json:"amount"`
	AmountFormatted   string     `json:"amount_formatted"`
	Product           string     `json:"product"`
	Installments      int        `json:"installments"`
	Status            string     `json:"status"`
	AuthorizationCode string     `json:"authorization_code,omitempty"`
	NSU               string     `json:"nsu,omitempty"`
	ResponseCode      string     `json:"response_code,omitempty"`
	CardBrand         string     `json:"card_brand"`
	CardLast4         string     `json:"card_last4"`
	CreatedAt         time.Time  `json:"created_at"`
	CapturedAt        *time.Time `json:"captured_at,omitempty"`
	CanceledAt        *time.Time `json:"canceled_at,omitempty"`
}

func toTransactionResponse(t *transaction.Transaction) TransactionResponse {
	return TransactionResponse{ID: t.ID(), MerchantID: t.MerchantID(), Amount: int64(t.Amount()), AmountFormatted: t.Amount().String(),
		Product: string(t.Product()), Installments: t.Installments(), Status: string(t.Status()), AuthorizationCode: t.AuthorizationCode(),
		NSU: t.NSU(), ResponseCode: t.ResponseCode(), CardBrand: t.Card().Brand, CardLast4: t.Card().Last4,
		CreatedAt: t.CreatedAt(), CapturedAt: t.CapturedAt(), CanceledAt: t.CanceledAt()}
}

type ReceivableResponse struct {
	ID             string `json:"id"`
	TransactionID  string `json:"transaction_id"`
	InstallmentNo  int    `json:"installment_no"`
	Installments   int    `json:"installments"`
	Product        string `json:"product"`
	Brand          string `json:"brand"`
	Gross          int64  `json:"gross_amount"`
	Fee            int64  `json:"fee_amount"`
	Net            int64  `json:"net_amount"`
	NetFormatted   string `json:"net_formatted"`
	DueDate        string `json:"due_date"`
	Status         string `json:"status"`
	SettlementID   string `json:"settlement_id,omitempty"`
	AnticipationID string `json:"anticipation_id,omitempty"`
}

func toReceivableResponse(r *receivable.Receivable) ReceivableResponse {
	return ReceivableResponse{ID: r.ID(), TransactionID: r.TransactionID(), InstallmentNo: r.InstallmentNo(), Installments: r.Installments(),
		Product: string(r.Product()), Brand: r.Brand(), Gross: int64(r.Gross()), Fee: int64(r.Fee()), Net: int64(r.Net()), NetFormatted: r.Net().String(),
		DueDate: r.DueDate().Format("2006-01-02"), Status: string(r.Status()), SettlementID: r.SettlementID(), AnticipationID: r.AnticipationID()}
}

func toReceivableResponses(list []*receivable.Receivable) []ReceivableResponse {
	out := make([]ReceivableResponse, 0, len(list))
	for _, r := range list {
		out = append(out, toReceivableResponse(r))
	}
	return out
}

type DailySummaryResponse struct {
	DueDate string `json:"due_date"`
	Status  string `json:"status"`
	Count   int    `json:"count"`
	Net     int64  `json:"net_amount"`
}

func toSummaryResponses(list []receivable.DailySummary) []DailySummaryResponse {
	out := make([]DailySummaryResponse, 0, len(list))
	for _, s := range list {
		out = append(out, DailySummaryResponse{DueDate: s.DueDate.Format("2006-01-02"), Status: string(s.Status), Count: s.Count, Net: int64(s.Net)})
	}
	return out
}

type AnticipationItemResponse struct {
	ReceivableID string `json:"receivable_id"`
	DueDate      string `json:"due_date"`
	Days         int    `json:"days"`
	Net          int64  `json:"net_amount"`
	PresentValue int64  `json:"present_value"`
	Discount     int64  `json:"discount"`
}

type AnticipationResponse struct {
	ID           string                     `json:"id"`
	MerchantID   string                     `json:"merchant_id"`
	Status       string                     `json:"status"`
	RateBpsMonth int                        `json:"rate_bps_month"`
	Gross        int64                      `json:"gross_amount"`
	Discount     int64                      `json:"discount_amount"`
	Net          int64                      `json:"net_amount"`
	NetFormatted string                     `json:"net_formatted"`
	RequestedAt  time.Time                  `json:"requested_at"`
	Items        []AnticipationItemResponse `json:"items"`
}

func toAnticipationResponse(a *anticipation.Anticipation) AnticipationResponse {
	items := make([]AnticipationItemResponse, 0, len(a.Items()))
	for _, it := range a.Items() {
		items = append(items, AnticipationItemResponse{ReceivableID: it.ReceivableID, DueDate: it.DueDate.Format("2006-01-02"), Days: it.Days,
			Net: int64(it.Net), PresentValue: int64(it.PresentValue), Discount: int64(it.Discount)})
	}
	return AnticipationResponse{ID: a.ID(), MerchantID: a.MerchantID(), Status: string(a.Status()), RateBpsMonth: int(a.RateMonth()),
		Gross: int64(a.Gross()), Discount: int64(a.Discount()), Net: int64(a.Net()), NetFormatted: a.Net().String(), RequestedAt: a.RequestedAt(), Items: items}
}

type SettlementResponse struct {
	ID          string    `json:"id"`
	Date        string    `json:"date"`
	Total       int64     `json:"total_amount"`
	ToMerchant  int64     `json:"to_merchant"`
	ToCreditors int64     `json:"to_creditors"`
	Retained    int64     `json:"retained"`
	Status      string    `json:"status"`
	ExternalRef string    `json:"external_ref,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

func toSettlementResponses(list []*settlement.Settlement) []SettlementResponse {
	out := make([]SettlementResponse, 0, len(list))
	for _, s := range list {
		out = append(out, SettlementResponse{ID: s.ID(), Date: s.Date().Format("2006-01-02"), Total: int64(s.Total()), ToMerchant: int64(s.ToMerchant()),
			ToCreditors: int64(s.ToCreditors()), Retained: int64(s.Retained()), Status: string(s.Status()), ExternalRef: s.ExternalRef(), CreatedAt: s.CreatedAt()})
	}
	return out
}

type ChargebackResponse struct {
	ID            string    `json:"id"`
	TransactionID string    `json:"transaction_id"`
	MerchantID    string    `json:"merchant_id"`
	ReasonCode    string    `json:"reason_code"`
	Amount        int64     `json:"amount"`
	Status        string    `json:"status"`
	OpenedAt      time.Time `json:"opened_at"`
	Deadline      time.Time `json:"deadline"`
}

func toChargebackResponse(c *chargeback.Chargeback) ChargebackResponse {
	return ChargebackResponse{ID: c.ID(), TransactionID: c.TransactionID(), MerchantID: c.MerchantID(), ReasonCode: c.ReasonCode(),
		Amount: int64(c.Amount()), Status: string(c.Status()), OpenedAt: c.OpenedAt(), Deadline: c.Deadline()}
}
