package handler

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	userpb "github.com/stilln0thing/GoKart/pkg/pb/user"
	db "github.com/stilln0thing/GoKart/services/user-service/internal/db"
	"github.com/stilln0thing/GoKart/services/user-service/internal/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UserGRPCHandler struct {
	userpb.UnimplementedUserServiceServer
	svc service.UserService
}

func NewUserGRPCHandler(svc service.UserService) *UserGRPCHandler {
	return &UserGRPCHandler{svc: svc}
}

// RegisterUser

func (h *UserGRPCHandler) RegisterUser(ctx context.Context, req *userpb.RegisterUserRequest) (*userpb.RegisterUserResponse, error) {
	if req.Email == "" || req.Username == "" || req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "email, username, and password are required")
	}

	user, err := h.svc.Register(ctx, req.Username, req.Email, req.Password)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to register user: %v", err)
	}

	return &userpb.RegisterUserResponse{
		User:    dbUserToProto(user),
		Message: "user registered successfully",
	}, nil
}

// LoginUser

func (h *UserGRPCHandler) LoginUser(ctx context.Context, req *userpb.LoginUserRequest) (*userpb.LoginUserResponse, error) {
	if req.Email == "" || req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "email and password are required")
	}

	token, user, err := h.svc.Login(ctx, req.Email, req.Password)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	return &userpb.LoginUserResponse{
		Token:   token,
		User:    dbUserToProto(user),
		Message: "login successful",
	}, nil
}

// GetUser

func (h *UserGRPCHandler) GetUser(ctx context.Context, req *userpb.GetUserRequest) (*userpb.GetUserResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "user id is required")
	}

	user, err := h.svc.GetUser(ctx, req.Id)
	if err != nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}

	return &userpb.GetUserResponse{
		User: dbUserToProto(user),
	}, nil
}

// UpdateUser

func (h *UserGRPCHandler) UpdateUser(ctx context.Context, req *userpb.UpdateUserRequest) (*userpb.UpdateUserResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "user id is required")
	}

	user, err := h.svc.UpdateUser(ctx, req.Id, req.Username, req.Email)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update user: %v", err)
	}

	return &userpb.UpdateUserResponse{
		User:    dbUserToProto(user),
		Message: "user updated successfully",
	}, nil
}

// Helpers

func dbUserToProto(u db.User) *userpb.User {
	return &userpb.User{
		Id:       uuidToString(u.ID),
		Email:    u.Email,
		Username: u.Username,
	}
}

func uuidToString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return uuid.UUID(id.Bytes).String()
}
