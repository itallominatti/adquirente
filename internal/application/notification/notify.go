package notification

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/itallominatti/adquirente/internal/application"
	"github.com/itallominatti/adquirente/internal/domain/merchant"
	"github.com/itallominatti/adquirente/internal/domain/shared"
)

type Sender interface {
	Send(ctx context.Context, url string, body []byte, headers map[string]string) error
}

type Notifier struct {
	merchants merchant.Repository
	sender    Sender
	secret    []byte
	log       *slog.Logger
}

func NewNotifier(merchants merchant.Repository, sender Sender, secret string, log *slog.Logger) *Notifier {
	return &Notifier{merchants: merchants, sender: sender, secret: []byte(secret), log: log}
}

type envelope struct {
	EventID    string `json:"event_id"`
	MerchantID string `json:"merchant_id"`
}

func (n *Notifier) Handle(ctx context.Context, eventType string, payload []byte) error {
	var env envelope
	if err := json.Unmarshal(payload, &env); err != nil || env.MerchantID == "" {
		return application.Permanent(fmt.Errorf("evento sem merchant_id: %w", err))
	}
	m, err := n.merchants.FindByID(ctx, env.MerchantID)
	if errors.Is(err, shared.ErrNotFound) {
		return application.Permanent(err)
	}
	if err != nil {
		return err // banco fora: transitório, o consumidor tenta de novo
	}
	if m.WebhookURL() == "" {
		return nil // EC não quer webhooks
	}
	headers := map[string]string{
		"Content-Type": "application/json",
		"X-Event-Id":   env.EventID,
		"X-Event-Type": eventType,
		"X-Signature":  "sha256=" + Sign(n.secret, payload),
	}
	if err := n.sender.Send(ctx, m.WebhookURL(), payload, headers); err != nil {
		return fmt.Errorf("entregar webhook %s ao EC %s: %w", env.EventID, m.ID(), err)
	}
	n.log.InfoContext(ctx, "webhook entregue", "event_id", env.EventID, "event_type", eventType, "merchant_id", m.ID())
	return nil
}

func Sign(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func Verify(secret, body []byte, signature string) bool {
	expected := Sign(secret, body)
	return hmac.Equal([]byte(expected), []byte(signature))
}
