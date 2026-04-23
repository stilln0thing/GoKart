package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	db "github.com/stilln0thing/GoKart/services/product-service/internal/db"
)

type EventType int

const (
	EventProductCreated EventType = iota
	EventInventoryUpdated
)

type ProductEvent struct {
	Type      EventType
	Product   db.Product // Used for ProductCreated
	ProductID string     // Used for InventoryUpdated
	Quantity  int32      // Used for InventoryUpdated
}

type ProductEventProducer interface {
	PublishProductCreated(ctx context.Context, product db.Product) error
	PublishInventoryUpdated(ctx context.Context, productID string, newQuantity int32) error
	Close() error
}

type KafkaWorker struct {
	queue  chan ProductEvent
	wg     sync.WaitGroup
	logger *slog.Logger
}

func NewKafkaWorker(bufferSize int, logger *slog.Logger) *KafkaWorker {
	return &KafkaWorker{
		queue:  make(chan ProductEvent, bufferSize),
		logger: logger.With("component", "product-kafka-worker"),
	}
}

func (k *KafkaWorker) Start(n int, producer ProductEventProducer) {
	for i := 0; i < n; i++ {
		k.wg.Add(1)
		go func(workerID int) {
			defer k.wg.Done()
			k.logger.Info("worker started", "worker_id", workerID)

			for event := range k.queue {
				// We use a small timeout context for producing events in background
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				var err error

				switch event.Type {
				case EventProductCreated:
					err = producer.PublishProductCreated(ctx, event.Product)
				case EventInventoryUpdated:
					err = producer.PublishInventoryUpdated(ctx, event.ProductID, event.Quantity)
				}

				cancel()

				if err != nil {
					k.logger.Error("kafka publish failed",
						"worker_id", workerID,
						"event_type", event.Type,
						"error", err,
					)
				}
			}

			k.logger.Info("worker stopped", "worker_id", workerID)
		}(i)
	}
}

func (k *KafkaWorker) Enqueue(event ProductEvent) bool {
	select {
	case k.queue <- event:
		return true
	default:
		k.logger.Warn("kafka worker queue full, dropping event",
			"event_type", event.Type,
		)
		return false
	}
}

func (k *KafkaWorker) Stop() {
	k.logger.Info("shutting down kafka workers, draining queue...",
		"pending", len(k.queue),
	)
	close(k.queue)
	k.wg.Wait()
	k.logger.Info("all kafka workers stopped")
}
