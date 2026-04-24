package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/stilln0thing/GoKart/services/order-service/internal/db"
)

type OrderService interface {
	CreateOrder(ctx context.Context, userID, productID string, quantity int32) (db.Order, error)
	GetOrder(ctx context.Context, id string) (db.Order, error)
	SubscribeOrderStatus(orderID string) <-chan db.Order
	UnsubscribeOrderStatus(orderID string, ch <-chan db.Order)
}

type orderService struct {
	queries     *db.Queries
	kafkaWorker *KafkaWorker
	logger      *slog.Logger

	// Local pub-sub for streaming updates
	mu        sync.RWMutex
	listeners map[string][]chan db.Order
}

func NewOrderService(queries *db.Queries, worker *KafkaWorker) OrderService {
	return &orderService{
		queries:     queries,
		kafkaWorker: worker,
		logger:      slog.Default().With("service", "order"),
		listeners:   make(map[string][]chan db.Order),
	}
}

func (s *orderService) CreateOrder(ctx context.Context, userID, productID string, quantity int32) (db.Order, error) {
	if userID == "" || productID == "" || quantity <= 0 {
		return db.Order{}, errors.New("invalid order data")
	}

	uid, err := uuid.Parse(userID)
	if err != nil {
		return db.Order{}, errors.New("invalid user id format")
	}

	pid, err := uuid.Parse(productID)
	if err != nil {
		return db.Order{}, errors.New("invalid product id format")
	}

	// Verify User & Product exist from our materialized views
	_, err = s.queries.GetUserViewByID(ctx, pgtype.UUID{Bytes: uid, Valid: true})
	if err != nil {
		return db.Order{}, fmt.Errorf("user not found or view not synced: %w", err)
	}

	prod, err := s.queries.GetProductViewByID(ctx, pgtype.UUID{Bytes: pid, Valid: true})
	if err != nil {
		return db.Order{}, fmt.Errorf("product not found or view not synced: %w", err)
	}

	if prod.Quantity < quantity {
		return db.Order{}, errors.New("insufficient product inventory in local view")
	}

	id := uuid.New()
	now := time.Now()

	order, err := s.queries.CreateOrder(ctx, db.CreateOrderParams{
		ID:        pgtype.UUID{Bytes: id, Valid: true},
		UserID:    pgtype.UUID{Bytes: uid, Valid: true},
		ProductID: pgtype.UUID{Bytes: pid, Valid: true},
		Quantity:  quantity,
		Status:    "PLACED",
		CreatedAt: pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return db.Order{}, fmt.Errorf("failed to create order: %w", err)
	}

	s.logger.Info("order created", "order_id", id.String())

	// Publish OrderPlaced event
	s.kafkaWorker.Enqueue(OrderEvent{
		Type:  EventOrderPlaced,
		Order: order,
	})

	// Simulate background order processing for streaming RPC
	go s.simulateOrderProcessing(id.String())

	return order, nil
}

func (s *orderService) GetOrder(ctx context.Context, id string) (db.Order, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return db.Order{}, fmt.Errorf("invalid order id: %w", err)
	}

	return s.queries.GetOrderByID(ctx, pgtype.UUID{Bytes: parsed, Valid: true})
}

// simulateOrderProcessing simulates status changes: PLACED -> CONFIRMED -> SHIPPED
func (s *orderService) simulateOrderProcessing(orderID string) {
	steps := []string{"CONFIRMED", "SHIPPED"}
	for _, status := range steps {
		time.Sleep(3 * time.Second) // Wait before transition

		ctx := context.Background()
		uid, _ := uuid.Parse(orderID)

		order, err := s.queries.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
			ID:     pgtype.UUID{Bytes: uid, Valid: true},
			Status: status,
		})
		if err == nil {
			s.logger.Info("order status updated", "order_id", orderID, "status", status)
			s.broadcast(orderID, order)
		}
	}
}

func (s *orderService) SubscribeOrderStatus(orderID string) <-chan db.Order {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan db.Order, 10)
	s.listeners[orderID] = append(s.listeners[orderID], ch)
	return ch
}

func (s *orderService) UnsubscribeOrderStatus(orderID string, ch <-chan db.Order) {
	s.mu.Lock()
	defer s.mu.Unlock()

	listeners := s.listeners[orderID]
	for i, listener := range listeners {
		if listener == ch {
			s.listeners[orderID] = append(listeners[:i], listeners[i+1:]...)
			close(listener)
			break
		}
	}

	if len(s.listeners[orderID]) == 0 {
		delete(s.listeners, orderID)
	}
}

func (s *orderService) broadcast(orderID string, order db.Order) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ch := range s.listeners[orderID] {
		select {
		case ch <- order:
		default:
			// channel full, drop this notification to not block
		}
	}
}
