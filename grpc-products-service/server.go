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

	"github.com/grpc-products-service/internal/controller"
	"github.com/grpc-products-service/internal/database"
	"github.com/grpc-products-service/internal/queue"
	"github.com/grpc-products-service/pb"
	"github.com/grpc-products-service/pkg"
	"github.com/grpc-products-service/pkg/helpers"
	"github.com/grpc-products-service/snowflake"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	cfg     pkg.Config
	workers = 5
)

func main() {
	ctx, cancel := context.WithCancel(context.Background()) // Changed to WithCancel for full shutdown control
	defer cancel()

	// Load config file
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

	// Database connection
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

	// Pulsar connection
	pulsarCfg := &queue.PulsarConfig{
		URI:   cfg.Queue.URI,
		Token: helpers.GetEnvOrDefault("PULSAR_TOKEN", ""),
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

	productController := controller.NewProductController(session)
	server := grpc.NewServer()

	// Enable reflection
	reflection.Register(server)
	pb.RegisterProductServiceServer(server, productController)

	// Graceful shutdown handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	stopCh := make(chan struct{}) // Define stopCh

	go func() {
		sig := <-sigChan
		slog.Info("Received shutdown signal", "signal", sig)
		slog.Info("Shutting down gRPC server...")

		// Gracefully stop the gRPC server
		server.GracefulStop()
		close(stopCh) // Signal ticker goroutine to stop
		cancel()      // Cancel context for other goroutines
		slog.Info("gRPC server has been stopped gracefully")
	}()

	// Background message processing (Fixed: Correctly stops on shutdown)
	for i := 0; i < workers; i++ {
		go helpers.ProcessMessages(ctx, session, producer, i, 10)

	}

	// Start server
	slog.Info("Starting gRPC server", "port", cfg.Server.Port)
	if err := server.Serve(lis); err != nil {
		slog.Error("gRPC server encountered an error while serving", "error", err)
		os.Exit(1)
	}
}
