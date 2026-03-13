package infra

import (
	"context"
	"github.com/segmentio/kafka-go"
	"encoding/json"
	"github.com/stilln0thing/GoKart/services/user-service/internal/service"
	"github.com/stilln0thing/GoKart/services/user-service/internal/domain"
)

type KafkaProducer struct {
	writer *kafka.Writer
}

func NewKafkaProducer(brokers []string, topic string) service.UserEventProducer {
	return &KafkaProducer{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Topic:    topic,
			Balancer: &kafka.LeastBytes{},
		},
	}
}

func (p *KafkaProducer) ProduceUserCreatedEvent(ctx context.Context, user *domain.User) error {
	event := struct {
		Type string `json:"type"`
		User *domain.User `json:"user"`
	}{
		Type: "UserCreated",
		User: user,
	}

	eventBytes, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(user.ID),
		Value: eventBytes,
	})
}