package main

import (
	"database/sql"
	"log"
	"os"
	"github.com/stilln0thing/GoKart/services/user-service/internal/infra"
	"github.com/stilln0thing/GoKart/services/user-service/internal/repository"
	"github.com/stilln0thing/GoKart/services/user-service/internal/service"
)

func main() {
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

	// Start gRPC server (not implemented in this snippet)
	// ...
	log.Println("User service started successfully", userService)
}