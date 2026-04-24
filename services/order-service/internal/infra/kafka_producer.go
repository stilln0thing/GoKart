package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	db "github.com/stilln0thing/GoKart/services/order-service/internal/db"
)

type KafkaProducer struct {
	writer *kafka.Writer
	logger *slog.Logger
}

func NewKafkaProducer(brokers []string) *KafkaProducer {
	return &KafkaProducer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Balancer:     &kafka.LeastBytes{},
			RequiredAcks: kafka.RequireOne,
			Async:        false,
		},
		logger: slog.Default().With("component", "order-kafka-producer"),
	}
}

func (p *KafkaProducer) PublishOrderPlaced(ctx context.Context, order db.Order) error {
	payload := struct {
		Type      string `json:"type"`
		OrderID   string `json:"order_id"`
		UserID    string `json:"user_id"`
		ProductID string `json:"product_id"`
		Quantity  int32  `json:"quantity"`
		Timestamp int64  `json:"timestamp"`
	}{
		Type:      "OrderPlaced",
		OrderID:   uuid.UUID(order.ID.Bytes).String(),
		UserID:    uuid.UUID(order.UserID.Bytes).String(),
		ProductID: uuid.UUID(order.ProductID.Bytes).String(),
		Quantity:  int32(order.Quantity),
		Timestamp: time.Now().UnixMilli(),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	msg := kafka.Message{
		Topic: "order-placed",
		Key:   []byte(payload.OrderID),
		Value: data,
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("failed to write kafka message: %w", err)
	}

	p.logger.Info("published OrderPlaced event", "order_id", payload.OrderID)
	return nil
}

func (p *KafkaProducer) Close() error {
	p.logger.Info("closing kafka producer")
	return p.writer.Close()
}
