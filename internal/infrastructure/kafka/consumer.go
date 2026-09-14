package kafka

import (
	"context"
	"errors"
	"log/slog"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/itallominatti/adquirente/internal/application"
)

type HandlerFunc func(ctx context.Context, eventType string, payload []byte) error

type Consumer struct {
	reader      *kafkago.Reader
	dlq         *kafkago.Writer
	handler     HandlerFunc
	log         *slog.Logger
	maxAttempts int
}

func NewConsumer(brokers []string, groupID string, topics []string, handler HandlerFunc, log *slog.Logger) *Consumer {
	return &Consumer{
		reader: kafkago.NewReader(kafkago.ReaderConfig{
			Brokers:     brokers,
			GroupID:     groupID,
			GroupTopics: topics,
			MinBytes:    1,
			MaxBytes:    10e6,
			MaxWait:     500 * time.Millisecond,
		}),
		dlq:         &kafkago.Writer{Addr: kafkago.TCP(brokers...), AllowAutoTopicCreation: true, RequiredAcks: kafkago.RequireAll},
		handler:     handler,
		log:         log,
		maxAttempts: 5,
	}
}

func (c *Consumer) Run(ctx context.Context) error {
	defer c.reader.Close()
	defer c.dlq.Close()
	for {
		msg, err := c.reader.FetchMessage(ctx) // não confirma ainda
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
		c.process(ctx, msg)
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			c.log.ErrorContext(ctx, "falha ao confirmar offset", "err", err)
		}
	}
}

func (c *Consumer) process(ctx context.Context, msg kafkago.Message) {
	eventType := header(msg, "event_type")
	if eventType == "" {
		eventType = msg.Topic
	}
	eventID := header(msg, "event_id")
	var lastErr error
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		lastErr = c.handler(ctx, eventType, msg.Value)
		if lastErr == nil {
			return
		}
		if application.IsPermanent(lastErr) {
			break
		}
		backoff := time.Duration(200*(1<<attempt)) * time.Millisecond // 400ms, 800ms, 1.6s, 3.2s, 6.4s
		c.log.WarnContext(ctx, "erro transitório; tentando de novo", "event_id", eventID, "attempt", attempt, "err", lastErr)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
	}
	c.log.ErrorContext(ctx, "evento enviado para a DLQ", "event_id", eventID, "topic", msg.Topic, "err", lastErr)
	dead := kafkago.Message{Topic: msg.Topic + ".dlq", Key: msg.Key, Value: msg.Value, Headers: msg.Headers}
	dead.Headers = append(dead.Headers, kafkago.Header{Key: "error", Value: []byte(lastErr.Error())})
	if err := c.dlq.WriteMessages(ctx, dead); err != nil {
		c.log.ErrorContext(ctx, "falha ao gravar na DLQ", "err", err)
	}
}

func header(msg kafkago.Message, key string) string {
	for _, h := range msg.Headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}
