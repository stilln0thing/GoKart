package service

import (
	"context"
	"github.com/stilln0thing/GoKart/services/user-service/internal/domain"
)

type UserEventProducer interface {
	ProduceUserCreatedEvent(ctx context.Context, user *domain.User) error
}
