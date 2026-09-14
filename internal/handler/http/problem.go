package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	appant "github.com/itallominatti/adquirente/internal/application/anticipation"
	apptx "github.com/itallominatti/adquirente/internal/application/transaction"
	"github.com/itallominatti/adquirente/internal/domain/anticipation"
	"github.com/itallominatti/adquirente/internal/domain/chargeback"
	"github.com/itallominatti/adquirente/internal/domain/merchant"
	"github.com/itallominatti/adquirente/internal/domain/shared"
	"github.com/itallominatti/adquirente/internal/domain/transaction"
)

type Problem struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance"`
	Code     string `json:"code"`
}

func problem(c *gin.Context, status int, code, detail string) {
	c.Header("Content-Type", "application/problem+json")
	c.AbortWithStatusJSON(status, Problem{
		Type: "https://api.adquirente.com/errors/" + code, Title: http.StatusText(status),
		Status: status, Detail: detail, Instance: c.Request.URL.Path, Code: code,
	})
}

var errorMap = []struct {
	err    error
	status int
	code   string
}{
	{shared.ErrNotFound, http.StatusNotFound, "NOT_FOUND"},
	{shared.ErrInvalidTransition, http.StatusConflict, "INVALID_TRANSITION"},
	{shared.ErrConcurrentUpdate, http.StatusConflict, "CONCURRENT_UPDATE"},
	{anticipation.ErrAlreadyDone, http.StatusConflict, "ALREADY_DONE"},

	{shared.ErrInvalidAmount, http.StatusBadRequest, "INVALID_AMOUNT"},
	{shared.ErrInvalidProduct, http.StatusBadRequest, "INVALID_PRODUCT"},
	{merchant.ErrInvalidDocument, http.StatusBadRequest, "INVALID_DOCUMENT"},
	{merchant.ErrInvalidBankAccount, http.StatusBadRequest, "INVALID_BANK_ACCOUNT"},
	{merchant.ErrInvalidLegalName, http.StatusBadRequest, "INVALID_LEGAL_NAME"},
	{merchant.ErrInvalidMCC, http.StatusBadRequest, "INVALID_MCC"},
	{merchant.ErrInvalidFeePlan, http.StatusBadRequest, "INVALID_FEE_PLAN"},
	{merchant.ErrInvalidWebhook, http.StatusBadRequest, "INVALID_WEBHOOK_URL"},
	{transaction.ErrInvalidPAN, http.StatusBadRequest, "INVALID_CARD"},
	{transaction.ErrInvalidToken, http.StatusBadRequest, "INVALID_CARD_TOKEN"},
	{transaction.ErrDebitInstallments, http.StatusBadRequest, "DEBIT_CANNOT_INSTALL"},
	{transaction.ErrMaxInstallments, http.StatusBadRequest, "INVALID_INSTALLMENTS"},
	{anticipation.ErrNoReceivables, http.StatusBadRequest, "NO_RECEIVABLES"},

	{merchant.ErrMerchantInactive, http.StatusUnprocessableEntity, "MERCHANT_INACTIVE"},
	{merchant.ErrInstallmentsNotAllowed, http.StatusUnprocessableEntity, "INSTALLMENTS_NOT_ALLOWED"},
	{apptx.ErrLimitExceeded, http.StatusUnprocessableEntity, "LIMIT_EXCEEDED"},
	{anticipation.ErrNotAnticipable, http.StatusUnprocessableEntity, "RECEIVABLE_NOT_ANTICIPABLE"},
	{anticipation.ErrWrongMerchant, http.StatusForbidden, "FORBIDDEN"},
	{appant.ErrReceivableLiened, http.StatusUnprocessableEntity, "RECEIVABLE_LIENED"},
	{chargeback.ErrInvalidAmount, http.StatusUnprocessableEntity, "INVALID_CHARGEBACK_AMOUNT"},
	{chargeback.ErrTransactionStatus, http.StatusUnprocessableEntity, "TRANSACTION_NOT_DISPUTABLE"},

	{apptx.ErrIssuerUnavailable, http.StatusServiceUnavailable, "ISSUER_UNAVAILABLE"},
	{context.DeadlineExceeded, http.StatusGatewayTimeout, "UPSTREAM_TIMEOUT"},
}

func respondError(c *gin.Context, err error) {
	for _, m := range errorMap {
		if errors.Is(err, m.err) {
			problem(c, m.status, m.code, err.Error())
			return
		}
	}
	// Erro inesperado: loga com detalhe, responde sem detalhe (não vaza internals).
	slog.ErrorContext(c.Request.Context(), "erro inesperado", "err", err, "path", c.Request.URL.Path)
	problem(c, http.StatusInternalServerError, "INTERNAL_ERROR", "erro interno do servidor")
}
