package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"

	db "github.com/stilln0thing/GoKart/services/user-service/internal/db"
)

// Service Interface

type UserService interface {
	Register(ctx context.Context, username, email, password string) (db.User, error)
	Login(ctx context.Context, email, password string) (string, db.User, error) // returns JWT token + user
	GetUser(ctx context.Context, id string) (db.User, error)
	UpdateUser(ctx context.Context, id, username, email string) (db.User, error)
}

// Implementation

type userService struct {
	queries     *db.Queries
	kafkaWorker *KafkaWorker
	jwtSecret   []byte
	logger      *slog.Logger
}

func NewUserService(queries *db.Queries, worker *KafkaWorker) UserService {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "gokart-dev-secret" // fallback for local dev
	}
	return &userService{
		queries:     queries,
		kafkaWorker: worker,
		jwtSecret:   []byte(secret),
		logger:      slog.Default().With("service", "user"),
	}
}

// Register

func (s *userService) Register(ctx context.Context, username, email, password string) (db.User, error) {
	if username == "" || email == "" || password == "" {
		return db.User{}, errors.New("username, email, and password are required")
	}

	// Check for duplicate email
	_, err := s.queries.GetUserByEmail(ctx, email)
	if err == nil {
		return db.User{}, fmt.Errorf("user with email %s already exists", email)
	}

	// Hash password
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return db.User{}, fmt.Errorf("failed to hash password: %w", err)
	}

	id := uuid.New()
	now := time.Now()

	user, err := s.queries.CreateUser(ctx, db.CreateUserParams{
		ID:        pgtype.UUID{Bytes: id, Valid: true},
		Username:  username,
		Email:     email,
		Password:  string(hashed),
		CreatedAt: pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return db.User{}, fmt.Errorf("failed to create user: %w", err)
	}

	s.logger.Info("user registered",
		"user_id", id.String(),
		"email", email,
	)

	// Enqueue Kafka event (non-blocking, best-effort)
	s.kafkaWorker.Enqueue(UserEvent{Type: EventUserRegistered, User: user})

	return user, nil
}

// Login

func (s *userService) Login(ctx context.Context, email, password string) (string, db.User, error) {
	user, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil {
		return "", db.User{}, fmt.Errorf("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return "", db.User{}, fmt.Errorf("invalid credentials")
	}

	// Build JWT
	uid := uuid.UUID(user.ID.Bytes)
	claims := jwt.MapClaims{
		"sub":   uid.String(),
		"email": user.Email,
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", db.User{}, fmt.Errorf("failed to sign token: %w", err)
	}

	s.logger.Info("user logged in", "user_id", uid.String())

	return signed, user, nil
}

// GetUser

func (s *userService) GetUser(ctx context.Context, id string) (db.User, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return db.User{}, fmt.Errorf("invalid user id: %w", err)
	}

	user, err := s.queries.GetUserByID(ctx, pgtype.UUID{Bytes: parsed, Valid: true})
	if err != nil {
		return db.User{}, fmt.Errorf("user not found: %w", err)
	}
	return user, nil
}

// UpdateUser

func (s *userService) UpdateUser(ctx context.Context, id, username, email string) (db.User, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return db.User{}, fmt.Errorf("invalid user id: %w", err)
	}

	user, err := s.queries.UpdateUser(ctx, db.UpdateUserParams{
		ID:       pgtype.UUID{Bytes: parsed, Valid: true},
		Username: username,
		Email:    email,
	})
	if err != nil {
		return db.User{}, fmt.Errorf("failed to update user: %w", err)
	}

	s.logger.Info("user updated", "user_id", id)

	// Enqueue Kafka event (non-blocking, best-effort)
	s.kafkaWorker.Enqueue(UserEvent{Type: EventUserUpdated, User: user})

	return user, nil
}
