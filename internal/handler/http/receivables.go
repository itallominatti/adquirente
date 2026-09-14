package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	apprcv "github.com/itallominatti/adquirente/internal/application/receivable"
	"github.com/itallominatti/adquirente/internal/domain/receivable"
	"github.com/itallominatti/adquirente/internal/domain/settlement"
)

type ReceivableHandler struct {
	query       *apprcv.Query
	settlements settlement.Repository
}

func NewReceivableHandler(query *apprcv.Query, settlements settlement.Repository) *ReceivableHandler {
	return &ReceivableHandler{query: query, settlements: settlements}
}

func (h *ReceivableHandler) List(c *gin.Context) {
	merchantID, ok := requireMerchant(c)
	if !ok {
		return
	}
	f := receivable.ListFilter{Status: receivable.Status(c.Query("status"))}
	var err error
	if f.From, err = parseDate(c.Query("from")); err != nil {
		problem(c, http.StatusBadRequest, "INVALID_DATE", "from deve ser YYYY-MM-DD")
		return
	}
	if f.To, err = parseDate(c.Query("to")); err != nil {
		problem(c, http.StatusBadRequest, "INVALID_DATE", "to deve ser YYYY-MM-DD")
		return
	}
	f.Limit, _ = strconv.Atoi(c.DefaultQuery("limit", "100"))
	list, err := h.query.List(c.Request.Context(), merchantID, f)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": toReceivableResponses(list)})
}

func (h *ReceivableHandler) Summary(c *gin.Context) {
	merchantID, ok := requireMerchant(c)
	if !ok {
		return
	}
	from, err1 := parseDate(c.DefaultQuery("from", time.Now().Format("2006-01-02")))
	to, err2 := parseDate(c.DefaultQuery("to", time.Now().AddDate(0, 3, 0).Format("2006-01-02")))
	if err1 != nil || err2 != nil {
		problem(c, http.StatusBadRequest, "INVALID_DATE", "from/to devem ser YYYY-MM-DD")
		return
	}
	list, err := h.query.Summary(c.Request.Context(), merchantID, from, to)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": toSummaryResponses(list)})
}

func (h *ReceivableHandler) ListSettlements(c *gin.Context) {
	merchantID, ok := requireMerchant(c)
	if !ok {
		return
	}
	list, err := h.settlements.ListByMerchant(c.Request.Context(), merchantID, 100)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": toSettlementResponses(list)})
}

func parseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse("2006-01-02", s)
}
