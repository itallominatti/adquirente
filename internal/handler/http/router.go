package http

import (
	"context"
	"crypto/rsa"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	"github.com/itallominatti/adquirente/internal/application"
	"github.com/itallominatti/adquirente/internal/handler/http/middleware"
	"github.com/itallominatti/adquirente/internal/infrastructure/observability"
)

type Deps struct {
	Log          *slog.Logger
	Metrics      *observability.Metrics
	PublicKey    *rsa.PublicKey
	JWTIssuer    string
	JWTAudience  string
	Idempotency  application.IdempotencyStore
	Ready        func(ctx context.Context) error // checa banco etc.
	Merchants    *MerchantHandler
	Transactions *TransactionHandler
	Receivables  *ReceivableHandler
	Anticipation *AnticipationHandler
	Chargebacks  *ChargebackHandler
	Prod         bool
}

func NewRouter(d Deps) *gin.Engine {
	if d.Prod {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery(), middleware.RequestID(), otelgin.Middleware("api"), middleware.Metrics(d.Metrics), middleware.Logger(d.Log))

	// Sem autenticação: saúde e métricas (em produção, /metrics fica só na rede interna).
	r.GET("/health/live", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	r.GET("/health/ready", func(c *gin.Context) {
		if err := d.Ready(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "reason": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
	r.GET("/metrics", gin.WrapH(d.Metrics.Handler()))
	if !d.Prod {
		r.StaticFile("/openapi.yaml", "api/openapi.yaml")
	}

	auth := middleware.Authenticate(d.PublicKey, d.JWTIssuer, d.JWTAudience)
	v1 := r.Group("/v1", auth)

	merchants := v1.Group("/merchants")
	merchants.POST("", middleware.Authorize("merchants:write"), d.Merchants.Create)
	merchants.GET("/:id", middleware.Authorize("merchants:read"), d.Merchants.Get)
	merchants.PATCH("/:id/settings", middleware.Authorize("merchants:write"), d.Merchants.UpdateSettings)
	merchants.POST("/:id/approve", middleware.Authorize("merchants:approve"), d.Merchants.Approve)
	merchants.POST("/:id/block", middleware.Authorize("merchants:approve"), d.Merchants.Block)

	v1.POST("/card-tokens", middleware.Authorize("transactions:write"), d.Transactions.Tokenize)

	tx := v1.Group("/transactions", middleware.Authorize("transactions:write"))
	tx.POST("", middleware.Idempotency(d.Idempotency), d.Transactions.Authorize)
	tx.POST("/:id/capture", middleware.Idempotency(d.Idempotency), d.Transactions.Capture)
	tx.POST("/:id/cancel", middleware.Idempotency(d.Idempotency), d.Transactions.Cancel)
	v1.GET("/transactions/:id", middleware.Authorize("transactions:read"), d.Transactions.Get)

	rcv := v1.Group("/receivables", middleware.Authorize("receivables:read"))
	rcv.GET("", d.Receivables.List)
	rcv.GET("/summary", d.Receivables.Summary)
	v1.GET("/settlements", middleware.Authorize("receivables:read"), d.Receivables.ListSettlements)

	ant := v1.Group("/anticipations", middleware.Authorize("anticipations:write"))
	ant.POST("/simulate", d.Anticipation.Simulate)
	ant.POST("", middleware.Idempotency(d.Idempotency), d.Anticipation.Create)
	ant.GET("/:id", d.Anticipation.Get)

	internal := r.Group("/internal", auth, middleware.Authorize("chargebacks:write"))
	internal.POST("/chargebacks", d.Chargebacks.Open)

	return r
}
