package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/stilln0thing/GoKart/services/product-service/internal/db"
)

type ProductService interface {
	CreateProduct(ctx context.Context, name, description string, price int64, quantity int32) (db.Product, error)
	GetProduct(ctx context.Context, id string) (db.Product, error)
	UpdateInventory(ctx context.Context, id string, delta int32) (db.Product, error)
}

type productService struct {
	queries     *db.Queries
	kafkaWorker *KafkaWorker
	logger      *slog.Logger
}

func NewProductService(queries *db.Queries, worker *KafkaWorker) ProductService {
	return &productService{
		queries:     queries,
		kafkaWorker: worker,
		logger:      slog.Default().With("service", "product"),
	}
}

func (s *productService) CreateProduct(ctx context.Context, name, description string, price int64, quantity int32) (db.Product, error) {
	if name == "" || price < 0 || quantity < 0 {
		return db.Product{}, errors.New("invalid product data")
	}

	id := uuid.New()
	now := time.Now()

	product, err := s.queries.CreateProduct(ctx, db.CreateProductParams{
		ID:          pgtype.UUID{Bytes: id, Valid: true},
		Name:        name,
		Description: description,
		Price:       price,
		Quantity:    int32(quantity),
		CreatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return db.Product{}, fmt.Errorf("failed to create product: %w", err)
	}

	s.logger.Info("product created", "product_id", id.String())

	// Publish async event
	s.kafkaWorker.Enqueue(ProductEvent{
		Type:    EventProductCreated,
		Product: product,
	})

	return product, nil
}

func (s *productService) GetProduct(ctx context.Context, id string) (db.Product, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return db.Product{}, fmt.Errorf("invalid product id: %w", err)
	}

	product, err := s.queries.GetProductByID(ctx, pgtype.UUID{Bytes: parsed, Valid: true})
	if err != nil {
		return db.Product{}, fmt.Errorf("product not found: %w", err)
	}

	return product, nil
}

func (s *productService) UpdateInventory(ctx context.Context, id string, delta int32) (db.Product, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return db.Product{}, fmt.Errorf("invalid product id: %w", err)
	}

	product, err := s.queries.UpdateInventory(ctx, db.UpdateInventoryParams{
		ID:    pgtype.UUID{Bytes: parsed, Valid: true},
		Delta: delta,
	})
	if err != nil {
		return db.Product{}, fmt.Errorf("failed to update inventory (insufficient stock?): %w", err)
	}

	s.logger.Info("inventory updated", "product_id", id, "delta", delta, "new_quantity", product.Quantity)

	// Publish async event
	s.kafkaWorker.Enqueue(ProductEvent{
		Type:      EventInventoryUpdated,
		ProductID: id,
		Quantity:  product.Quantity,
	})

	return product, nil
}
