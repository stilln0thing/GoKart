package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	db "github.com/stilln0thing/GoKart/services/order-service/internal/db"
)

type EventType int

const (
	EventOrderPlaced EventType = iota
)

type OrderEvent struct {
	Type  EventType
	Order db.Order
}

type OrderEventProducer interface {
	PublishOrderPlaced(ctx context.Context, order db.Order) error
	Close() error
}

type KafkaWorker struct {
	queue  chan OrderEvent
	wg     sync.WaitGroup
	logger *slog.Logger
}

func NewKafkaWorker(bufferSize int, logger *slog.Logger) *KafkaWorker {
	return &KafkaWorker{
		queue:  make(chan OrderEvent, bufferSize),
		logger: logger.With("component", "order-kafka-worker"),
	}
}

func (k *KafkaWorker) Start(n int, producer OrderEventProducer) {
	for i := 0; i < n; i++ {
		k.wg.Add(1)
		go func(workerID int) {
			defer k.wg.Done()
			k.logger.Info("worker started", "worker_id", workerID)

			for event := range k.queue {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				var err error

				switch event.Type {
				case EventOrderPlaced:
					err = producer.PublishOrderPlaced(ctx, event.Order)
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

func (k *KafkaWorker) Enqueue(event OrderEvent) bool {
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
