package main

import (
	"database/sql"
	"log"
	"os"
	"net"
	"syscall"
	"os/signal"
	_ "github.com/lib/pq"

	"github.com/joho/godotenv"
	"github.com/stilln0thing/GoKart/services/user-service/internal/infra"
	"github.com/stilln0thing/GoKart/services/user-service/internal/repository"
	"github.com/stilln0thing/GoKart/services/user-service/internal/service"
	"github.com/stilln0thing/GoKart/services/user-service/internal/handler"
	userpb "github.com/stilln0thing/GoKart/pkg/pb/user"
	"google.golang.org/grpc"
)

func main() {

	// Load env
	if err := godotenv.Load(); err != nil {
		log.Println("Could not load env file")
	}

	// Initialise db
	db, err := sql.Open("postgres", os.Getenv("POSTGRES_DSN"))
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Initialise infra (Kafka producer)
	kafkaProducer := infra.NewKafkaProducer(
		[]string{os.Getenv("KAFKA_BROKER")},
		"user-events",
	)

	// Initialise layers
	userRepo := repository.NewPostgresRepo(db)
	userService := service.NewUserService(userRepo, kafkaProducer)

	grpcServer := grpc.NewServer()
	userHandler := handler.NewUserGRPCHandler(userService)
	userpb.RegisterUserServiceServer(grpcServer, userHandler)

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	go func() {
		log.Println("gRPC server listening on :50051")
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	// Wait for Termination signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop // This blocks until a signal is received
	log.Println("Shutting down gracefully...")
	
	// Cleanup operations
	grpcServer.GracefulStop()
	db.Close()
	// kafkaProducer.Close()
	
	log.Println("Service stopped.")
	
}