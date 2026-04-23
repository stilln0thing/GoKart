package service

import (
	"context"

	db "github.com/stilln0thing/GoKart/services/user-service/internal/db"
)

// UserEventProducer is the port for publishing user domain events to Kafka.
type UserEventProducer interface {
	PublishUserRegistered(ctx context.Context, user db.User) error
	Close() error
}
