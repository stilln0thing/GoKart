package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"

	db "github.com/stilln0thing/GoKart/services/user-service/internal/db"
	"github.com/stilln0thing/GoKart/services/user-service/internal/service"
)

// Compile-time check: KafkaProducer implements service.UserEventProducer.
var _ service.UserEventProducer = (*KafkaProducer)(nil)

type KafkaProducer struct {
	writer *kafka.Writer
	logger *slog.Logger
}

func NewKafkaProducer(brokers []string, topic string) *KafkaProducer {
	return &KafkaProducer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        topic,
			Balancer:     &kafka.LeastBytes{},
			RequiredAcks: kafka.RequireOne,
			Async:        false, // synchronous writes — we want delivery confirmation
		},
		logger: slog.Default().With("component", "kafka-producer"),
	}
}

// PublishUserRegistered publishes a UserRegistered event to Kafka.
func (p *KafkaProducer) PublishUserRegistered(ctx context.Context, user db.User) error {
	uid := uuid.UUID(user.ID.Bytes)

	payload := struct {
		Type      string `json:"type"`
		UserID    string `json:"user_id"`
		Username  string `json:"username"`
		Email     string `json:"email"`
		Timestamp int64  `json:"timestamp"`
	}{
		Type:      "UserRegistered",
		UserID:    uid.String(),
		Username:  user.Username,
		Email:     user.Email,
		Timestamp: time.Now().UnixMilli(),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	msg := kafka.Message{
		Key:   []byte(uid.String()),
		Value: data,
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("failed to write kafka message: %w", err)
	}

	p.logger.Info("published UserRegistered event",
		"user_id", uid.String(),
		"topic", p.writer.Topic,
	)
	return nil
}

// Close flushes and closes the Kafka writer. Called during graceful shutdown.
func (p *KafkaProducer) Close() error {
	p.logger.Info("closing kafka producer")
	return p.writer.Close()
}
