package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env         string // dev | prod
	ServiceName string
	LogLevel    string

	HTTPAddr string
	GRPCAddr string

	DatabaseURL  string
	RedisAddr    string
	KafkaBrokers []string

	IssuerAddr    string
	VaultAddr     string
	IssuerTimeout time.Duration
	MaxTxAmount   int64 // teto por transação, em centavos

	JWTPublicKeyFile  string
	JWTPrivateKeyFile string // só o auth-sim
	JWTIssuer         string
	JWTAudience       string

	TLSCertFile string
	TLSKeyFile  string
	TLSCAFile   string

	VaultKeyBase64 string // só o vault

	OTLPEndpoint string

	SettlementOutputDir string
	RegistryLiensFile   string
	SettlementDate      string // YYYY-MM-DD; vazio = hoje

	WebhookSecret string

	IssuerSimLatencyMs int
}

func Load() Config {
	return Config{
		Env:         getenv("APP_ENV", "dev"),
		ServiceName: getenv("SERVICE_NAME", "adquirente"),
		LogLevel:    getenv("LOG_LEVEL", "info"),

		HTTPAddr: getenv("HTTP_ADDR", ":8080"),
		GRPCAddr: getenv("GRPC_ADDR", ":9090"),

		DatabaseURL:  getenv("DATABASE_URL", "postgres://adq:adq@localhost:5432/adquirente?sslmode=disable"),
		RedisAddr:    getenv("REDIS_ADDR", "localhost:6379"),
		KafkaBrokers: strings.Split(getenv("KAFKA_BROKERS", "localhost:9092"), ","),

		IssuerAddr:    getenv("ISSUER_ADDR", "localhost:9091"),
		VaultAddr:     getenv("VAULT_ADDR", "localhost:9092"),
		IssuerTimeout: duration("ISSUER_TIMEOUT", 2*time.Second),
		MaxTxAmount:   integer("MAX_TX_AMOUNT", 5_000_000), // R$ 50.000,00

		JWTPublicKeyFile:  getenv("JWT_PUBLIC_KEY_FILE", "certs/jwt-public.pem"),
		JWTPrivateKeyFile: getenv("JWT_PRIVATE_KEY_FILE", "certs/jwt-private.pem"),
		JWTIssuer:         getenv("JWT_ISSUER", "https://auth.adquirente.local"),
		JWTAudience:       getenv("JWT_AUDIENCE", "adquirente-api"),

		TLSCertFile: os.Getenv("TLS_CERT_FILE"),
		TLSKeyFile:  os.Getenv("TLS_KEY_FILE"),
		TLSCAFile:   os.Getenv("TLS_CA_FILE"),

		VaultKeyBase64: os.Getenv("VAULT_KEY_BASE64"),

		OTLPEndpoint: os.Getenv("OTLP_ENDPOINT"), // ex.: otel-collector:4317; vazio desliga o tracing

		SettlementOutputDir: getenv("SETTLEMENT_OUTPUT_DIR", "./out/settlements"),
		RegistryLiensFile:   os.Getenv("REGISTRY_LIENS_FILE"),
		SettlementDate:      os.Getenv("SETTLEMENT_DATE"),

		WebhookSecret: getenv("WEBHOOK_SECRET", "dev-secret-change-me"),

		IssuerSimLatencyMs: int(integer("ISSUER_SIM_LATENCY_MS", 50)),
	}
}

func (c Config) IsProd() bool { return c.Env == "prod" }

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func integer(key string, def int64) int64 {
	if v, err := strconv.ParseInt(os.Getenv(key), 10, 64); err == nil {
		return v
	}
	return def
}

func duration(key string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(os.Getenv(key)); err == nil {
		return v
	}
	return def
}
