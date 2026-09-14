package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	appmerchant "github.com/itallominatti/adquirente/internal/application/merchant"
	"github.com/itallominatti/adquirente/internal/domain/merchant"
	"github.com/itallominatti/adquirente/internal/handler/http/middleware"
)

type MerchantHandler struct {
	onboard   *appmerchant.Onboard
	review    *appmerchant.Review
	merchants merchant.Repository
}

func NewMerchantHandler(onboard *appmerchant.Onboard, review *appmerchant.Review, merchants merchant.Repository) *MerchantHandler {
	return &MerchantHandler{onboard: onboard, review: review, merchants: merchants}
}

func (h *MerchantHandler) Create(c *gin.Context) {
	var req CreateMerchantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		problem(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	m, err := h.onboard.Execute(c.Request.Context(), appmerchant.OnboardCommand{
		Document: req.Document, LegalName: req.LegalName, MCC: req.MCC,
		BankCode: req.Bank.Code, Branch: req.Bank.Branch, AccountNumber: req.Bank.Number, AccountKind: req.Bank.Kind,
		DebitBps: req.FeePlan.DebitBps, CreditBps: req.FeePlan.CreditBps, CreditInstallmentBps: req.FeePlan.CreditInstallmentBps,
		AnticipationBpsMonth: req.FeePlan.AnticipationBpsMonth, WebhookURL: req.WebhookURL,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.Header("Location", "/v1/merchants/"+m.ID())
	c.JSON(http.StatusCreated, toMerchantResponse(m))
}

func (h *MerchantHandler) Get(c *gin.Context) {
	id, ok := ownedMerchantID(c, c.Param("id"))
	if !ok {
		return
	}
	m, err := h.merchants.FindByID(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toMerchantResponse(m))
}

func (h *MerchantHandler) Approve(c *gin.Context) {
	m, err := h.review.Approve(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toMerchantResponse(m))
}

func (h *MerchantHandler) Block(c *gin.Context) {
	m, err := h.review.Block(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toMerchantResponse(m))
}

func (h *MerchantHandler) UpdateSettings(c *gin.Context) {
	id, ok := ownedMerchantID(c, c.Param("id"))
	if !ok {
		return
	}
	var req UpdateMerchantSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		problem(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	m, err := h.review.UpdateSettings(c.Request.Context(), appmerchant.UpdateSettingsCommand{MerchantID: id, WebhookURL: req.WebhookURL, AutoAnticipation: req.AutoAnticipation})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toMerchantResponse(m))
}

func ownedMerchantID(c *gin.Context, urlID string) (string, bool) {
	p := middleware.PrincipalFrom(c)
	if p.MerchantID != "" && p.MerchantID != urlID {
		problem(c, http.StatusForbidden, "FORBIDDEN", "você não pode acessar outro estabelecimento")
		return "", false
	}
	return urlID, true
}

func requireMerchant(c *gin.Context) (string, bool) {
	p := middleware.PrincipalFrom(c)
	if p.MerchantID == "" {
		problem(c, http.StatusForbidden, "FORBIDDEN", "esta operação exige credencial de estabelecimento")
		return "", false
	}
	return p.MerchantID, true
}
