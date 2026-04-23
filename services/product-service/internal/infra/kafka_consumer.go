package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/segmentio/kafka-go"
	"github.com/stilln0thing/GoKart/services/product-service/internal/service"
)

type KafkaConsumer struct {
	reader  *kafka.Reader
	dlq     *kafka.Writer // used to send unprocessable messages to DLQ
	service service.ProductService
	logger  *slog.Logger
}

func NewKafkaConsumer(brokers []string, svc service.ProductService) *KafkaConsumer {
	return &KafkaConsumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:  brokers,
			GroupID:  "product-service-group", // important for horizontal scaling
			Topic:    "order-placed",
			MinBytes: 10e3, // 10KB
			MaxBytes: 10e6, // 10MB
		}),
		dlq: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        "dlq-topic",
			Balancer:     &kafka.LeastBytes{},
			RequiredAcks: kafka.RequireOne,
		},
		service: svc,
		logger:  slog.Default().With("component", "kafka-consumer"),
	}
}

// Start consuming events blocking. Call this in a goroutine.
func (c *KafkaConsumer) Start(ctx context.Context) {
	c.logger.Info("started consuming from order-placed topic")

	for {
		m, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return // Context cancelled, shutdown cleanly
			}
			c.logger.Error("failed to fetch message", "error", err)
			continue
		}

		if err := c.processMessage(ctx, m); err != nil {
			c.logger.Error("failed to process message, routing to DLQ", "error", err, "topic", m.Topic, "partition", m.Partition, "offset", m.Offset)
			c.routeToDLQ(ctx, m, err.Error())
		}

		if err := c.reader.CommitMessages(ctx, m); err != nil {
			c.logger.Error("failed to commit message", "error", err)
		}
	}
}

func (c *KafkaConsumer) processMessage(ctx context.Context, m kafka.Message) error {
	// The expected payload format from order-service when an order is placed.
	// Since we defined the architecture, we know it will contain productId and quantity.
	type OrderPlacedEvent struct {
		Type      string `json:"type"`
		OrderID   string `json:"order_id"`
		ProductID string `json:"product_id"`
		Quantity  int32  `json:"quantity"`
	}

	var event OrderPlacedEvent
	if err := json.Unmarshal(m.Value, &event); err != nil {
		return fmt.Errorf("malformed JSON payload: %w", err)
	}

	if event.Type != "OrderPlaced" {
		c.logger.Warn("ignored unknown event type", "type", event.Type)
		return nil
	}

	c.logger.Info("processing order-placed event", "order_id", event.OrderID, "product_id", event.ProductID, "quantity", event.Quantity)

	// Deduct the inventory by passing a negative delta
	_, err := c.service.UpdateInventory(ctx, event.ProductID, -event.Quantity)
	if err != nil {
		return fmt.Errorf("could not update inventory: %w", err)
	}

	return nil
}

func (c *KafkaConsumer) routeToDLQ(ctx context.Context, m kafka.Message, reason string) {
	// Re-construct headers, add the reason and original topic
	headers := m.Headers
	headers = append(headers, kafka.Header{Key: "error_reason", Value: []byte(reason)})
	headers = append(headers, kafka.Header{Key: "original_topic", Value: []byte(m.Topic)})

	dlqMsg := kafka.Message{
		Key:     m.Key,
		Value:   m.Value,
		Headers: headers,
	}

	if err := c.dlq.WriteMessages(ctx, dlqMsg); err != nil {
		c.logger.Error("failed to write to DLQ. Event lost!", "error", err)
	} else {
		c.logger.Info("successfully routed message to DLQ")
	}
}

func (c *KafkaConsumer) Close() error {
	c.logger.Info("closing kafka consumer")
	_ = c.dlq.Close()
	return c.reader.Close()
}
