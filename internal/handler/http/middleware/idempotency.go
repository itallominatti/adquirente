package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/itallominatti/adquirente/internal/application"
)

func Idempotency(store application.IdempotencyStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("Idempotency-Key")
		if key == "" || len(key) > 128 {
			abortProblem(c, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "envie o header Idempotency-Key (um UUID por operação)")
			return
		}
		p := PrincipalFrom(c)
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
		if err != nil {
			abortProblem(c, http.StatusBadRequest, "INVALID_BODY", "não foi possível ler o corpo")
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body)) // devolve o corpo para o handler ler

		sum := sha256.Sum256(append([]byte(c.Request.Method+" "+c.Request.URL.Path+"\n"), body...))
		hash := hex.EncodeToString(sum[:])

		res, err := store.Begin(c.Request.Context(), p.MerchantID, key, hash)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "idempotência indisponível", "err", err)
			abortProblem(c, http.StatusServiceUnavailable, "IDEMPOTENCY_UNAVAILABLE", "tente novamente")
			return
		}
		switch res.State {
		case application.IdempotencyReplay:
			c.Header("Idempotent-Replayed", "true")
			c.Data(res.Status, "application/json", res.Body)
			c.Abort()
			return
		case application.IdempotencyConflict:
			abortProblem(c, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "esta chave já foi usada com um corpo diferente")
			return
		case application.IdempotencyInProgress:
			abortProblem(c, http.StatusConflict, "REQUEST_IN_PROGRESS", "a requisição original ainda está em processamento")
			return
		}

		w := &bodyCapture{ResponseWriter: c.Writer, buf: &bytes.Buffer{}}
		c.Writer = w
		c.Next()

		status := c.Writer.Status()
		if status >= 500 {
			_ = store.Abandon(c.Request.Context(), p.MerchantID, key)
			return
		}
		if err := store.Complete(c.Request.Context(), p.MerchantID, key, status, w.buf.Bytes()); err != nil {
			slog.ErrorContext(c.Request.Context(), "guardar resposta idempotente", "err", err)
		}
	}
}

type bodyCapture struct {
	gin.ResponseWriter
	buf *bytes.Buffer
}

func (w *bodyCapture) Write(b []byte) (int, error) {
	w.buf.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *bodyCapture) WriteString(s string) (int, error) {
	w.buf.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

func abortProblem(c *gin.Context, status int, code, detail string) {
	c.Header("Content-Type", "application/problem+json")
	c.AbortWithStatusJSON(status, gin.H{"status": status, "title": http.StatusText(status), "code": code, "detail": detail, "instance": c.Request.URL.Path})
}
