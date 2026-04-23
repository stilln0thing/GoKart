package service

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/stilln0thing/GoKart/services/user-service/internal/domain"
	"github.com/stilln0thing/GoKart/services/user-service/internal/repository"
)

type UserService interface {
	CreateUser(ctx context.Context, user *domain.User) (*domain.User, error)
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	// UpdateUser(ctx context.Context, user *domain.User) error
	// DeleteUser(ctx context.Context, id string) error
}

type userService struct {
	repo          repository.UserRepository
	eventProducer UserEventProducer
}

func NewUserService(repo repository.UserRepository, eventProducer UserEventProducer) *userService {
	return &userService{repo: repo, eventProducer: eventProducer}
}

func (s *userService) CreateUser(ctx context.Context, user *domain.User) (*domain.User, error) {
	user.ID = uuid.New().String()
	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now

	if err := user.Validate(); err != nil {
		return nil, err
	}
	log.Println("Creating user in database...")
	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	// Produce Kafka event here (e.g., UserCreated)
	return user, nil
}

func (s *userService) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	return s.repo.GetUserByEmail(ctx, email)
}
