package middleware

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/itallominatti/adquirente/internal/infrastructure/observability"
)

func Logger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		status := c.Writer.Status()
		attrs := []any{
			"method", c.Request.Method,
			"route", c.FullPath(),
			"status", status,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", c.GetString(RequestIDKey),
			"client_ip", c.ClientIP(),
		}
		if p, ok := c.Get(PrincipalKey); ok {
			attrs = append(attrs, "client_id", p.(Principal).ClientID, "merchant_id", p.(Principal).MerchantID)
		}
		switch {
		case status >= 500:
			log.ErrorContext(c.Request.Context(), "request", attrs...)
		case status >= 400:
			log.WarnContext(c.Request.Context(), "request", attrs...)
		default:
			log.InfoContext(c.Request.Context(), "request", attrs...)
		}
	}
}

func Metrics(m *observability.Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched" // evita cardinalidade explodir com URLs inválidas
		}
		m.HTTPRequests.WithLabelValues(route, c.Request.Method, strconv.Itoa(c.Writer.Status())).Inc()
		m.HTTPDuration.WithLabelValues(route, c.Request.Method).Observe(time.Since(start).Seconds())
	}
}
