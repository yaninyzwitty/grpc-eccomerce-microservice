package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/yaninyzwitty/grpc-inventory-service/helpers"
	"github.com/yaninyzwitty/grpc-inventory-service/internal/controller"
	"github.com/yaninyzwitty/grpc-inventory-service/internal/database"
	"github.com/yaninyzwitty/grpc-inventory-service/pb"
	"github.com/yaninyzwitty/grpc-inventory-service/pkg"
	"github.com/yaninyzwitty/grpc-inventory-service/queue"
	"github.com/yaninyzwitty/grpc-inventory-service/snowflake"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var cfg pkg.Config
	file, err := os.Open("config.yaml")
	if err != nil {
		slog.Error("failed to open config.yaml", "error", err)
		os.Exit(1)
	}
	defer file.Close()
	if err := cfg.LoadConfig(file); err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)

	}

	// Load .env variables
	if err := godotenv.Load(); err != nil {
		slog.Error("failed to load .env file", "error", err)
		os.Exit(1)
	}

	astraConfig := &database.AstraConfig{
		Username: cfg.Database.Username,
		Path:     cfg.Database.Path,
		Token:    helpers.GetEnvOrDefault("ASTRA_TOKEN", ""),
	}

	db := database.NewAstraDB()
	session, err := db.Connect(ctx, astraConfig, 30*time.Second)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer session.Close() // Close session only after server shutdown
	pulsarCfg := &queue.PulsarConfig{
		URI:       cfg.Queue.URI,
		TopicName: cfg.Queue.TopicName,
		Token:     helpers.GetEnvOrDefault("PULSAR_TOKEN", ""),
	}
	pulsarClient, err := pulsarCfg.CreatePulsarConnection(ctx)
	if err != nil {
		slog.Error("failed to create pulsar connection", "error", err)
		os.Exit(1)
	}
	defer pulsarClient.Close()

	producer, err := pulsarCfg.CreatePulsarProducer(ctx, pulsarClient)
	if err != nil {
		slog.Error("failed to create pulsar producer", "error", err)
		os.Exit(1)
	}
	defer producer.Close()

	// Initialize Snowflake
	err = snowflake.InitSonyFlake()
	if err != nil {
		slog.Error("failed to initialize snowflake", "error", err)
		os.Exit(1)
	}

	// Start gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Server.Port))
	if err != nil {
		slog.Error("failed to listen", "error", err)
		os.Exit(1)
	}

	inventoryController := controller.NewInventoryController(session)
	server := grpc.NewServer()
	reflection.Register(server)
	pb.RegisterInventoryServiceServer(server, inventoryController)
	// Graceful shutdown handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	// stopCH := make(chan os.Signal, 1)

	go func() {
		sig := <-sigChan
		slog.Info("Received shutdown signal", "signal", sig)
		slog.Info("Shutting down gRPC server...")

		// Gracefully stop the gRPC server
		server.GracefulStop()
		cancel() // Cancel context for other goroutines
		slog.Info("gRPC server has been stopped gracefully")
	}()
	// polling approach

	// go func() {
	// 	ticker := time.NewTicker(4 * time.Second)
	// 	defer ticker.Stop()

	// 	for {
	// 		select {
	// 		case <-ticker.C:
	// 			// poll messages
	// 			if err := helpers.ProcessMessages(context.Background(), session, producer); err != nil {
	// 				slog.Error("failed to process messages", "error", err)
	// 				os.Exit(1)
	// 			}
	// 		case <-stopCH:
	// 			return
	// 		}

	// 	}
	// }()

	// Start server
	slog.Info("Starting gRPC server", "port", cfg.Server.Port)
	if err := server.Serve(lis); err != nil {
		slog.Error("gRPC server encountered an error while serving", "error", err)
		os.Exit(1)
	}

}
