package main

import (
	"crypto/rsa"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/itallominatti/adquirente/internal/infrastructure/config"
	"github.com/itallominatti/adquirente/internal/infrastructure/observability"
)

type client struct {
	Secret     string `json:"secret"`
	MerchantID string `json:"merchant_id"`
	Scope      string `json:"scope"`
}

func main() {
	cfg := config.Load()
	log := observability.NewLogger(cfg.LogLevel, "auth-sim")

	keyPEM, err := os.ReadFile(cfg.JWTPrivateKeyFile)
	must(log, err, "ler chave privada")
	priv, err := jwt.ParseRSAPrivateKeyFromPEM(keyPEM)
	must(log, err, "chave privada inválida")

	raw, err := os.ReadFile(getenv("AUTH_CLIENTS_FILE", "certs/clients.json"))
	must(log, err, "ler clients.json")
	var clients map[string]client
	must(log, json.Unmarshal(raw, &clients), "clients.json inválido")

	r := gin.New()
	r.Use(gin.Recovery())

	r.POST("/oauth/token", func(c *gin.Context) {
		if c.PostForm("grant_type") != "client_credentials" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported_grant_type"})
			return
		}
		id, secret := c.PostForm("client_id"), c.PostForm("client_secret")
		cl, ok := clients[id]
		// Comparação em tempo constante: não vaza, pelo tempo de resposta, quantos caracteres acertou.
		if !ok || subtle.ConstantTimeCompare([]byte(cl.Secret), []byte(secret)) != 1 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_client"})
			return
		}
		now := time.Now()
		claims := jwt.MapClaims{
			"iss": cfg.JWTIssuer, "aud": cfg.JWTAudience, "sub": id,
			"iat": now.Unix(), "exp": now.Add(15 * time.Minute).Unix(), // curto: token vazado vale pouco
			"scope": cl.Scope,
		}
		if cl.MerchantID != "" {
			claims["merchant_id"] = cl.MerchantID
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tok.Header["kid"] = "dev-key-1"
		signed, err := tok.SignedString(priv)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"access_token": signed, "token_type": "Bearer", "expires_in": 900, "scope": cl.Scope})
	})

	// JWKS: a chave pública em formato padrão, para quem quiser validar tokens sem arquivo PEM.
	r.GET("/.well-known/jwks.json", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"keys": []gin.H{jwk(&priv.PublicKey, "dev-key-1")}})
	})

	log.Info("auth-sim ouvindo", "addr", cfg.HTTPAddr)
	must(log, (&http.Server{Addr: cfg.HTTPAddr, Handler: r, ReadHeaderTimeout: 5 * time.Second}).ListenAndServe(), "servidor")
}

func jwk(pub *rsa.PublicKey, kid string) gin.H {
	return gin.H{
		"kty": "RSA", "use": "sig", "alg": "RS256", "kid": kid,
		"n": base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func must(log *slog.Logger, err error, what string) {
	if err != nil {
		log.Error(what, "err", err)
		os.Exit(1)
	}
}
