package handler

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	productpb "github.com/stilln0thing/GoKart/pkg/pb/product"
	db "github.com/stilln0thing/GoKart/services/product-service/internal/db"
	"github.com/stilln0thing/GoKart/services/product-service/internal/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ProductGRPCHandler struct {
	productpb.UnimplementedProductServiceServer
	svc service.ProductService
}

func NewProductGRPCHandler(svc service.ProductService) *ProductGRPCHandler {
	return &ProductGRPCHandler{svc: svc}
}

func (h *ProductGRPCHandler) CreateProduct(ctx context.Context, req *productpb.CreateProductRequest) (*productpb.CreateProductResponse, error) {
	if req.Name == "" || req.Price < 0 || req.Quantity < 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid product data")
	}

	product, err := h.svc.CreateProduct(ctx, req.Name, req.Description, req.Price, req.Quantity)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create product: %v", err)
	}

	return &productpb.CreateProductResponse{
		Product: dbProductToProto(product),
	}, nil
}

func (h *ProductGRPCHandler) GetProduct(ctx context.Context, req *productpb.GetProductRequest) (*productpb.GetProductResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "product id required")
	}

	product, err := h.svc.GetProduct(ctx, req.Id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "product not found: %v", err)
	}

	return &productpb.GetProductResponse{
		Product: dbProductToProto(product),
	}, nil
}

func (h *ProductGRPCHandler) UpdateInventory(ctx context.Context, req *productpb.UpdateInventoryRequest) (*productpb.UpdateInventoryResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "product id required")
	}

	product, err := h.svc.UpdateInventory(ctx, req.Id, req.Delta)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update inventory: %v", err)
	}

	return &productpb.UpdateInventoryResponse{
		Product: dbProductToProto(product),
	}, nil
}

func dbProductToProto(p db.Product) *productpb.Product {
	return &productpb.Product{
		Id:          uuidToString(p.ID),
		Name:        p.Name,
		Description: p.Description,
		Price:       p.Price,
		Quantity:    p.Quantity,
	}
}

func uuidToString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return uuid.UUID(id.Bytes).String()
}
