package service

import (
	"context"
	"log/slog"
	"sync"

	db "github.com/stilln0thing/GoKart/services/user-service/internal/db"
)

// EventType distinguishes the kind of Kafka event to publish.
type EventType int

const (
	EventUserRegistered EventType = iota
	EventUserUpdated
)

// UserEvent is the unit of work pushed into the worker queue.
type UserEvent struct {
	Type EventType
	User db.User
}

// KafkaWorker is a buffered worker pool that publishes Kafka events asynchronously.
type KafkaWorker struct {
	queue  chan UserEvent
	wg     sync.WaitGroup
	logger *slog.Logger
}

// NewKafkaWorker creates a worker with the given buffer size.
func NewKafkaWorker(bufferSize int, logger *slog.Logger) *KafkaWorker {
	return &KafkaWorker{
		queue:  make(chan UserEvent, bufferSize),
		logger: logger.With("component", "kafka-worker"),
	}
}

// Start launches n goroutines that consume events from the queue and publish them via the producer.
func (k *KafkaWorker) Start(n int, producer UserEventProducer) {
	for i := 0; i < n; i++ {
		k.wg.Add(1)
		go func(workerID int) {
			defer k.wg.Done()
			k.logger.Info("worker started", "worker_id", workerID)

			for event := range k.queue {
				var err error
				switch event.Type {
				case EventUserRegistered:
					err = producer.PublishUserRegistered(context.Background(), event.User)
				case EventUserUpdated:
					err = producer.PublishUserUpdated(context.Background(), event.User)
				}
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

// Enqueue pushes an event into the buffered queue.
func (k *KafkaWorker) Enqueue(event UserEvent) bool {
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

// Stop closes the queue and waits for all in-flight workers to finish.
// Call this during graceful shutdown.
func (k *KafkaWorker) Stop() {
	k.logger.Info("shutting down kafka workers, draining queue...",
		"pending", len(k.queue),
	)
	close(k.queue)
	k.wg.Wait()
	k.logger.Info("all kafka workers stopped")
}
