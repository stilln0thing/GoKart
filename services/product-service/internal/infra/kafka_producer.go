package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	db "github.com/stilln0thing/GoKart/services/product-service/internal/db"
)

// KafkaProducer handles publishing product events.
type KafkaProducer struct {
	writer *kafka.Writer
	logger *slog.Logger
}

// NewKafkaProducer creates a new kafka producer for product events.
func NewKafkaProducer(brokers []string) *KafkaProducer {
	return &KafkaProducer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Balancer:     &kafka.LeastBytes{},
			RequiredAcks: kafka.RequireOne,
			Async:        false, // Synchronous writes for delivery confirmation
		},
		logger: slog.Default().With("component", "product-kafka-producer"),
	}
}

// PublishProductCreated publishes a ProductCreated event to 'product-created' topic
func (p *KafkaProducer) PublishProductCreated(ctx context.Context, product db.Product) error {
	uid := uuid.UUID(product.ID.Bytes)

	payload := struct {
		Type        string `json:"type"`
		ProductID   string `json:"product_id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Price       int64  `json:"price"`
		Quantity    int    `json:"quantity"`
		Timestamp   int64  `json:"timestamp"`
	}{
		Type:        "ProductCreated",
		ProductID:   uid.String(),
		Name:        product.Name,
		Description: product.Description,
		Price:       product.Price,
		Quantity:    int(product.Quantity),
		Timestamp:   time.Now().UnixMilli(),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	msg := kafka.Message{
		Topic: "product-created",
		Key:   []byte(uid.String()),
		Value: data,
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("failed to write kafka message: %w", err)
	}

	p.logger.Info("published ProductCreated event", "product_id", uid.String())
	return nil
}

// PublishInventoryUpdated publishes an InventoryUpdated event to 'inventory-updated' topic
func (p *KafkaProducer) PublishInventoryUpdated(ctx context.Context, productID string, newQuantity int32) error {
	payload := struct {
		Type        string `json:"type"`
		ProductID   string `json:"product_id"`
		NewQuantity int32  `json:"new_quantity"`
		Timestamp   int64  `json:"timestamp"`
	}{
		Type:        "InventoryUpdated",
		ProductID:   productID,
		NewQuantity: newQuantity,
		Timestamp:   time.Now().UnixMilli(),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	msg := kafka.Message{
		Topic: "inventory-updated",
		Key:   []byte(productID),
		Value: data,
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("failed to write kafka message: %w", err)
	}

	p.logger.Info("published InventoryUpdated event", "product_id", productID, "new_quantity", newQuantity)
	return nil
}

func (p *KafkaProducer) Close() error {
	p.logger.Info("closing kafka producer")
	return p.writer.Close()
}
