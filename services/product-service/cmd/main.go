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
	productpb "github.com/stilln0thing/GoKart/pkg/pb/product"
	db "github.com/stilln0thing/GoKart/services/product-service/internal/db"
	"github.com/stilln0thing/GoKart/services/product-service/internal/handler"
	"github.com/stilln0thing/GoKart/services/product-service/internal/infra"
	"github.com/stilln0thing/GoKart/services/product-service/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	// Structured Logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load .env
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
	kafkaProducer := infra.NewKafkaProducer([]string{kafkaBroker})

	// Kafka Worker Pool
	kafkaWorker := service.NewKafkaWorker(100, logger)
	kafkaWorker.Start(3, kafkaProducer) // start 3 worker goroutines

	// Service + Handler
	productService := service.NewProductService(queries, kafkaWorker)
	productHandler := handler.NewProductGRPCHandler(productService)

	// Kafka Consumer
	kafkaConsumer := infra.NewKafkaConsumer([]string{kafkaBroker}, productService)
	consumerCtx, consumerCancel := context.WithCancel(ctx)
	go kafkaConsumer.Start(consumerCtx)

	// gRPC Server
	grpcServer := grpc.NewServer()
	productpb.RegisterProductServiceServer(grpcServer, productHandler)
	reflection.Register(grpcServer)

	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		logger.Error("failed to listen", "error", err)
		os.Exit(1)
	}

	go func() {
		logger.Info("gRPC server listening", "port", ":50052")
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("gRPC server failed", "error", err)
			os.Exit(1)
		}
	}()

	// TODO: Phase 4: Handle Kafka Consumer (order-placed event) here

	// Graceful Shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	sig := <-stop

	logger.Info("received shutdown signal", "signal", sig.String())
	logger.Info("draining in-flight requests...")

	// Stop accepting new RPCs
	grpcServer.GracefulStop()
	logger.Info("gRPC server stopped")

	// Stop Kafka Consumer first so we don't pick up new events
	consumerCancel()
	if err := kafkaConsumer.Close(); err != nil {
		logger.Error("failed to close kafka consumer", "error", err)
	}

	// Stop Kafka workers natively
	kafkaWorker.Stop()

	// Close Kafka producer
	if err := kafkaProducer.Close(); err != nil {
		logger.Error("failed to close kafka producer", "error", err)
	}
	logger.Info("kafka producer closed")

	// Close Postgres pool
	pool.Close()
	logger.Info("postgres pool closed")

	logger.Info("product-service shutdown complete")
}
