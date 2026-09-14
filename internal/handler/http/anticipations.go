package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	appant "github.com/itallominatti/adquirente/internal/application/anticipation"
	appcb "github.com/itallominatti/adquirente/internal/application/chargeback"
)

type AnticipationHandler struct{ uc *appant.Anticipate }

func NewAnticipationHandler(uc *appant.Anticipate) *AnticipationHandler {
	return &AnticipationHandler{uc: uc}
}

func (h *AnticipationHandler) Simulate(c *gin.Context) {
	merchantID, ok := requireMerchant(c)
	if !ok {
		return
	}
	var req AnticipationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		problem(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	a, err := h.uc.Simulate(c.Request.Context(), merchantID, req.ReceivableIDs)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toAnticipationResponse(a))
}

func (h *AnticipationHandler) Create(c *gin.Context) {
	merchantID, ok := requireMerchant(c)
	if !ok {
		return
	}
	var req AnticipationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		problem(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	a, err := h.uc.Request(c.Request.Context(), merchantID, req.ReceivableIDs)
	if err != nil {
		respondError(c, err)
		return
	}
	c.Header("Location", "/v1/anticipations/"+a.ID())
	c.JSON(http.StatusCreated, toAnticipationResponse(a))
}

func (h *AnticipationHandler) Get(c *gin.Context) {
	merchantID, ok := requireMerchant(c)
	if !ok {
		return
	}
	a, err := h.uc.Get(c.Request.Context(), merchantID, c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toAnticipationResponse(a))
}

type ChargebackHandler struct{ open *appcb.Open }

func NewChargebackHandler(open *appcb.Open) *ChargebackHandler { return &ChargebackHandler{open: open} }

func (h *ChargebackHandler) Open(c *gin.Context) {
	var req OpenChargebackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		problem(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	cb, err := h.open.Execute(c.Request.Context(), appcb.OpenCommand{MerchantID: req.MerchantID, TransactionID: req.TransactionID, ReasonCode: req.ReasonCode, Amount: req.Amount})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toChargebackResponse(cb))
}
