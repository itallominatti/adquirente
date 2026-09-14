package middleware

import (
	"crypto/rsa"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const PrincipalKey = "principal"

type Principal struct {
	ClientID   string
	MerchantID string
	Scopes     map[string]bool
}

func (p Principal) Has(scope string) bool { return p.Scopes[scope] }

type Claims struct {
	jwt.RegisteredClaims
	MerchantID string `json:"merchant_id,omitempty"`
	Scope      string `json:"scope"`
}

func LoadPublicKey(path string) (*rsa.PublicKey, error) {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ler chave pública: %w", err)
	}
	return jwt.ParseRSAPublicKeyFromPEM(pem)
}

func Authenticate(pub *rsa.PublicKey, issuer, audience string) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if raw == "" || raw == c.GetHeader("Authorization") {
			unauthorized(c, "token ausente")
			return
		}
		claims := &Claims{}
		tok, err := jwt.ParseWithClaims(raw, claims, func(*jwt.Token) (any, error) { return pub, nil },
			jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
			jwt.WithIssuer(issuer),
			jwt.WithAudience(audience),
			jwt.WithExpirationRequired(),
		)
		if err != nil || !tok.Valid {
			unauthorized(c, "token inválido")
			return
		}
		scopes := map[string]bool{}
		for _, s := range strings.Fields(claims.Scope) {
			scopes[s] = true
		}
		c.Set(PrincipalKey, Principal{ClientID: claims.Subject, MerchantID: claims.MerchantID, Scopes: scopes})
		c.Next()
	}
}

func Authorize(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !PrincipalFrom(c).Has(scope) {
			c.Header("Content-Type", "application/problem+json")
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"status": 403, "title": "Forbidden", "code": "FORBIDDEN",
				"detail": "escopo insuficiente: " + scope, "instance": c.Request.URL.Path})
		}
	}
}

func PrincipalFrom(c *gin.Context) Principal {
	if p, ok := c.Get(PrincipalKey); ok {
		return p.(Principal)
	}
	return Principal{Scopes: map[string]bool{}}
}

func unauthorized(c *gin.Context, detail string) {
	c.Header("WWW-Authenticate", `Bearer realm="adquirente"`)
	c.Header("Content-Type", "application/problem+json")
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status": 401, "title": "Unauthorized", "code": "UNAUTHENTICATED",
		"detail": detail, "instance": c.Request.URL.Path})
}
