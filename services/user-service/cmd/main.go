package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	userpb "github.com/stilln0thing/GoKart/pkg/pb/user"
	db "github.com/stilln0thing/GoKart/services/user-service/internal/db"
	"github.com/stilln0thing/GoKart/services/user-service/internal/handler"
	"github.com/stilln0thing/GoKart/services/user-service/internal/infra"
	"github.com/stilln0thing/GoKart/services/user-service/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	// Structured Logger 
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load .env (local dev only)
	if err := godotenv.Load(); err != nil {
		logger.Warn("could not load .env file, using environment variables")
	}

	// PostgreSQL
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("POSTGRES_DSN"))
	if err != nil {
		logger.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	if err := pool.Ping(ctx); err != nil {
		logger.Error("failed to ping postgres", "error", err)
		os.Exit(1)
	}
	logger.Info("connected to postgres")

	// sqlc Queries
	queries := db.New(pool)

	// Kafka Producer
	kafkaBroker := os.Getenv("KAFKA_BROKER")
	if kafkaBroker == "" {
		kafkaBroker = "localhost:9092"
	}
	kafkaProducer := infra.NewKafkaProducer(
		[]string{kafkaBroker},
		"user-registered",
	)

	// Service + Handler
	userService := service.NewUserService(queries, kafkaProducer)
	userHandler := handler.NewUserGRPCHandler(userService)

	// gRPC Server
	grpcServer := grpc.NewServer()
	userpb.RegisterUserServiceServer(grpcServer, userHandler)
	reflection.Register(grpcServer)

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		logger.Error("failed to listen", "error", err)
		os.Exit(1)
	}

	go func() {
		logger.Info("gRPC server listening", "port", ":50051")
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("gRPC server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful Shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	sig := <-stop

	logger.Info("received shutdown signal", "signal", sig.String())
	logger.Info("draining in-flight requests...")

	// Stop accepting new RPCs and wait for in-flight to finish
	grpcServer.GracefulStop()
	logger.Info("gRPC server stopped")

	// Close Kafka producer (flush pending writes)
	if err := kafkaProducer.Close(); err != nil {
		logger.Error("failed to close kafka producer", "error", err)
	}
	logger.Info("kafka producer closed")

	// Close Postgres connection pool
	pool.Close()
	logger.Info("postgres pool closed")

	logger.Info("user-service shutdown complete")
}
