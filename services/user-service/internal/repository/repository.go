package repository

import (
	"context"
	"github.com/stilln0thing/GoKart/services/user-service/internal/domain"
)

type UserRepository interface {
	CreateUser(ctx context.Context, user *domain.User) error
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	// UpdateUser(ctx context.Context, user *domain.User) error
	// DeleteUser(ctx context.Context, id string) error
}