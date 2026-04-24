package handler

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	orderpb "github.com/stilln0thing/GoKart/pkg/pb/order"
	db "github.com/stilln0thing/GoKart/services/order-service/internal/db"
	"github.com/stilln0thing/GoKart/services/order-service/internal/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type OrderGRPCHandler struct {
	orderpb.UnimplementedOrderServiceServer
	svc service.OrderService
}

func NewOrderGRPCHandler(svc service.OrderService) *OrderGRPCHandler {
	return &OrderGRPCHandler{svc: svc}
}

func (h *OrderGRPCHandler) CreateOrder(ctx context.Context, req *orderpb.CreateOrderRequest) (*orderpb.CreateOrderResponse, error) {
	if req.UserId == "" || req.ProductId == "" || req.Quantity <= 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid order data")
	}

	order, err := h.svc.CreateOrder(ctx, req.UserId, req.ProductId, req.Quantity)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create order: %v", err)
	}

	return &orderpb.CreateOrderResponse{
		Order:   dbOrderToProto(order),
		Message: "order created successfully",
	}, nil
}

func (h *OrderGRPCHandler) GetOrder(ctx context.Context, req *orderpb.GetOrderRequest) (*orderpb.GetOrderResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "order id required")
	}

	order, err := h.svc.GetOrder(ctx, req.Id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "order not found: %v", err)
	}

	return &orderpb.GetOrderResponse{
		Order: dbOrderToProto(order),
	}, nil
}

func (h *OrderGRPCHandler) StreamOrderStatus(req *orderpb.StreamOrderStatusRequest, stream orderpb.OrderService_StreamOrderStatusServer) error {
	if req.Id == "" {
		return status.Error(codes.InvalidArgument, "order id required")
	}

	// Fetch current status and send immediately
	initial, err := h.svc.GetOrder(stream.Context(), req.Id)
	if err != nil {
		return status.Errorf(codes.NotFound, "order not found: %v", err)
	}

	if err := stream.Send(&orderpb.OrderStatusUpdate{
		Id:        req.Id,
		Status:    initial.Status,
		UpdatedAt: initial.UpdatedAt.Time.UnixMilli(),
	}); err != nil {
		return err
	}

	// If it's already finished, return
	if initial.Status == "SHIPPED" || initial.Status == "CANCELLED" || initial.Status == "DELIVERED" {
		return nil
	}

	ch := h.svc.SubscribeOrderStatus(req.Id)
	defer h.svc.UnsubscribeOrderStatus(req.Id, ch)

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case order, ok := <-ch:
			if !ok {
				return nil
			}
			err := stream.Send(&orderpb.OrderStatusUpdate{
				Id:        req.Id,
				Status:    order.Status,
				UpdatedAt: order.UpdatedAt.Time.UnixMilli(),
			})
			if err != nil {
				return err
			}
			if order.Status == "SHIPPED" || order.Status == "CANCELLED" || order.Status == "DELIVERED" {
				return nil
			}
		}
	}
}

func dbOrderToProto(o db.Order) *orderpb.Order {
	return &orderpb.Order{
		Id:        uuidToString(o.ID),
		UserId:    uuidToString(o.UserID),
		ProductId: uuidToString(o.ProductID),
		Quantity:  o.Quantity,
		Status:    o.Status,
		CreatedAt: o.CreatedAt.Time.UnixMilli(),
		UpdatedAt: o.UpdatedAt.Time.UnixMilli(),
	}
}

func uuidToString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return uuid.UUID(id.Bytes).String()
}
