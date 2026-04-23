package handler

import (
	"context"
	"log"

	userpb "github.com/stilln0thing/GoKart/pkg/pb/user"
	"github.com/stilln0thing/GoKart/services/user-service/internal/domain"
	"github.com/stilln0thing/GoKart/services/user-service/internal/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UserGRPCHandler struct {
	userpb.UnimplementedUserServiceServer
	service service.UserService
}

func NewUserGRPCHandler(svc service.UserService) *UserGRPCHandler {
	return &UserGRPCHandler{
		service: svc,
	}
}

func (h *UserGRPCHandler) CreateUser(ctx context.Context, req *userpb.CreateUserRequest) (*userpb.CreateUserResponse, error) {
	user := &domain.User{
		Email:    req.Email,
		Username: req.Username,
	}
	log.Println("Creating user...")
	createdUser, err := h.service.CreateUser(ctx, user)
	if err != nil {
		return nil, status.Error(codes.Internal, "Failed to create user")
	}

	pbUser := &userpb.User{
		Id:       createdUser.ID,
		Email:    createdUser.Email,
		Username: createdUser.Username,
	}

	return &userpb.CreateUserResponse{
		User:    pbUser,
		Message: "User created successfully",
	}, nil
}
