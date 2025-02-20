package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/yaninyzwitty/grpc-inventory-service/helpers"
	"github.com/yaninyzwitty/grpc-inventory-service/internal/database"
	"github.com/yaninyzwitty/grpc-inventory-service/pkg"
	"github.com/yaninyzwitty/grpc-inventory-service/queue"
	"github.com/yaninyzwitty/grpc-inventory-service/snowflake"
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

}
