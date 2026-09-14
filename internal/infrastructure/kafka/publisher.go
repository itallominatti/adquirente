package kafka

import (
	"context"
	"fmt"

	kafkago "github.com/segmentio/kafka-go"
)

type Publisher struct{ w *kafkago.Writer }

func NewPublisher(brokers []string) *Publisher {
	return &Publisher{w: &kafkago.Writer{
		Addr:                   kafkago.TCP(brokers...),
		Balancer:               &kafkago.Hash{},    // mesma chave → mesma partição → ordem por EC
		RequiredAcks:           kafkago.RequireAll, // acks=all: só confirma quando todas as réplicas gravaram
		AllowAutoTopicCreation: true,               // conveniência de desenvolvimento; em produção, tópicos são criados por IaC
	}}
}

func (p *Publisher) Publish(ctx context.Context, topic, key string, value []byte, headers map[string]string) error {
	msg := kafkago.Message{Topic: topic, Key: []byte(key), Value: value}
	for k, v := range headers {
		msg.Headers = append(msg.Headers, kafkago.Header{Key: k, Value: []byte(v)})
	}
	if err := p.w.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("publicar em %s: %w", topic, err)
	}
	return nil
}

func (p *Publisher) Close() error { return p.w.Close() }
