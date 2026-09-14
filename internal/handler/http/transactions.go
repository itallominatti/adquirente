package http

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	apptx "github.com/itallominatti/adquirente/internal/application/transaction"
	"github.com/itallominatti/adquirente/internal/domain/transaction"
	"github.com/itallominatti/adquirente/internal/infrastructure/grpcclient"
	"github.com/itallominatti/adquirente/internal/infrastructure/observability"
)

type TransactionHandler struct {
	authorize *apptx.Authorize
	lifecycle *apptx.Lifecycle
	get       *apptx.Get
	vault     *grpcclient.VaultClient
	metrics   *observability.Metrics
}

func NewTransactionHandler(authorize *apptx.Authorize, lifecycle *apptx.Lifecycle, get *apptx.Get, vault *grpcclient.VaultClient, metrics *observability.Metrics) *TransactionHandler {
	return &TransactionHandler{authorize: authorize, lifecycle: lifecycle, get: get, vault: vault, metrics: metrics}
}

func (h *TransactionHandler) Tokenize(c *gin.Context) {
	var req TokenizeCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		problem(c, http.StatusBadRequest, "INVALID_REQUEST", "cartão inválido") // nunca ecoa o corpo
		return
	}
	card, err := h.vault.Tokenize(c.Request.Context(), req.PAN, req.Expiry)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, CardTokenResponse{Token: card.Token, Brand: card.Brand, BIN: card.BIN, Last4: card.Last4})
}

func (h *TransactionHandler) Authorize(c *gin.Context) {
	merchantID, ok := requireMerchant(c)
	if !ok {
		return
	}
	var req AuthorizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		problem(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	tx, err := h.authorize.Execute(c.Request.Context(), apptx.AuthorizeCommand{
		MerchantID: merchantID, Amount: req.Amount, Product: req.Product, Installments: req.Installments, CardToken: req.CardToken,
	})
	if err != nil {
		h.metrics.Authorizations.WithLabelValues(req.Product, "error").Inc()
		respondError(c, err)
		return
	}
	h.metrics.Authorizations.WithLabelValues(req.Product, strings.ToLower(string(tx.Status()))).Inc()
	// Negada também é 201: o recurso "transação" foi criado, com status DENIED e response_code.
	c.Header("Location", "/v1/transactions/"+tx.ID())
	c.JSON(http.StatusCreated, toTransactionResponse(tx))
}

func (h *TransactionHandler) Get(c *gin.Context) {
	merchantID, ok := requireMerchant(c)
	if !ok {
		return
	}
	tx, err := h.get.Execute(c.Request.Context(), merchantID, c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toTransactionResponse(tx))
}

func (h *TransactionHandler) Capture(c *gin.Context) { h.change(c, h.lifecycle.Capture) }

func (h *TransactionHandler) Cancel(c *gin.Context) { h.change(c, h.lifecycle.Cancel) }

func (h *TransactionHandler) change(c *gin.Context, fn func(ctx context.Context, merchantID, id string) (*transaction.Transaction, error)) {
	merchantID, ok := requireMerchant(c)
	if !ok {
		return
	}
	tx, err := fn(c.Request.Context(), merchantID, c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toTransactionResponse(tx))
}
