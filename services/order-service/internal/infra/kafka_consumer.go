package infra

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/segmentio/kafka-go"
	db "github.com/stilln0thing/GoKart/services/order-service/internal/db"
)

type CQRSKafkaConsumer struct {
	reader  *kafka.Reader
	queries *db.Queries
	logger  *slog.Logger
}

func NewCQRSKafkaConsumer(brokers []string, queries *db.Queries) *CQRSKafkaConsumer {
	return &CQRSKafkaConsumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:     brokers,
			GroupID:     "order-service-cqrs-group",
			GroupTopics: []string{"user-registered", "product-created", "inventory-updated"},
			MinBytes:    10e3, // 10KB
			MaxBytes:    10e6, // 10MB
		}),
		queries: queries,
		logger:  slog.Default().With("component", "order-cqrs-consumer"),
	}
}

func (c *CQRSKafkaConsumer) Start(ctx context.Context) {
	c.logger.Info("started consuming user/product events for materialized view")

	for {
		m, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return // Context cancelled, shutdown cleanly
			}
			c.logger.Error("failed to fetch message", "error", err)
			continue
		}

		c.processMessage(ctx, m)

		if err := c.reader.CommitMessages(ctx, m); err != nil {
			c.logger.Error("failed to commit message", "error", err)
		}
	}
}

func (c *CQRSKafkaConsumer) processMessage(ctx context.Context, m kafka.Message) {
	// Attempt to unmarshal based on expected payload `Type` struct roughly
	var baseEvent struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(m.Value, &baseEvent); err != nil {
		c.logger.Error("could not parse base event type", "error", err)
		return
	}

	switch baseEvent.Type {
	case "UserRegistered", "UserUpdated":
		c.handleUserEvent(ctx, m.Value)
	case "ProductCreated":
		c.handleProductCreatedEvent(ctx, m.Value)
	case "InventoryUpdated":
		c.handleInventoryUpdatedEvent(ctx, m.Value)
	}
}

func (c *CQRSKafkaConsumer) handleUserEvent(ctx context.Context, data []byte) {
	var event struct {
		UserID   string `json:"user_id"`
		Username string `json:"username"`
		Email    string `json:"email"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		c.logger.Error("failed to unmarshal user event", "error", err)
		return
	}

	uid, err := uuid.Parse(event.UserID)
	if err != nil {
		return
	}

	err = c.queries.UpsertUserView(ctx, db.UpsertUserViewParams{
		ID:       pgtype.UUID{Bytes: uid, Valid: true},
		Username: event.Username,
		Email:    event.Email,
	})
	if err != nil {
		c.logger.Error("failed to upsert user view", "user_id", event.UserID, "error", err)
	} else {
		c.logger.Info("upserted user view", "user_id", event.UserID)
	}
}

func (c *CQRSKafkaConsumer) handleProductCreatedEvent(ctx context.Context, data []byte) {
	var event struct {
		ProductID string `json:"product_id"`
		Name      string `json:"name"`
		Price     int64  `json:"price"`
		Quantity  int    `json:"quantity"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		c.logger.Error("failed to unmarshal product event", "error", err)
		return
	}

	uid, err := uuid.Parse(event.ProductID)
	if err != nil {
		return
	}

	err = c.queries.UpsertProductView(ctx, db.UpsertProductViewParams{
		ID:       pgtype.UUID{Bytes: uid, Valid: true},
		Name:     event.Name,
		Price:    event.Price,
		Quantity: int32(event.Quantity),
	})
	if err != nil {
		c.logger.Error("failed to upsert product view", "product_id", event.ProductID, "error", err)
	} else {
		c.logger.Info("upserted product view", "product_id", event.ProductID)
	}
}

func (c *CQRSKafkaConsumer) handleInventoryUpdatedEvent(ctx context.Context, data []byte) {
	var event struct {
		ProductID   string `json:"product_id"`
		NewQuantity int32  `json:"new_quantity"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		return
	}
	uid, err := uuid.Parse(event.ProductID)
	if err != nil {
		return
	}

	// For simplicity, we just fetch existing and upsert with new quantity
	prod, err := c.queries.GetProductViewByID(ctx, pgtype.UUID{Bytes: uid, Valid: true})
	if err != nil {
		c.logger.Error("product not found in view for inventory update", "product_id", event.ProductID)
		return
	}

	err = c.queries.UpsertProductView(ctx, db.UpsertProductViewParams{
		ID:       pgtype.UUID{Bytes: uid, Valid: true},
		Name:     prod.Name,
		Price:    prod.Price,
		Quantity: event.NewQuantity,
	})
	if err != nil {
		c.logger.Error("failed to update product view inventory", "product_id", event.ProductID, "error", err)
	} else {
		c.logger.Info("updated product view inventory", "product_id", event.ProductID, "new_quantity", event.NewQuantity)
	}
}

func (c *CQRSKafkaConsumer) Close() error {
	c.logger.Info("closing cqrs kafka consumer")
	return c.reader.Close()
}
