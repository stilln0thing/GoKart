package service

import (
	"context"
	"github.com/stilln0thing/GoKart/services/user-service/internal/domain"
	"github.com/stilln0thing/GoKart/services/user-service/internal/repository"
)

type UswerService interface {
	CreateUser(ctx context.Context, user *domain.User) error
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	UpdateUser(ctx context.Context, user *domain.User) error
	DeleteUser(ctx context.Context, id string) error
}

type UserService struct {
	repo repository.UserRepository
	eventProducer UserEventProducer
}

func NewUserService(repo repository.UserRepository, eventProducer UserEventProducer) *UserService {
	return &UserService{repo: repo, eventProducer: eventProducer}
}

func (s *UserService) CreateUser(ctx context.Context, user *domain.User) (*domain.User, error) {
	if err := user.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	// Produce Kafka event here (e.g., UserCreated)
	return user, nil
}
