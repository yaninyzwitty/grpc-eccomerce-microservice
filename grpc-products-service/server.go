package main

import (
	"log/slog"
	"os"

	"github.com/grpc-products-service/internal/database"
	"github.com/grpc-products-service/pkg"
	"github.com/grpc-products-service/pkg/helpers"
	"github.com/grpc-products-service/snowflake"
	"github.com/joho/godotenv"
)

func main() {
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

	err = snowflake.InitSonyFlake()
	if err != nil {
		slog.Error("failed to initialize snowflake", "error", err)
		os.Exit(1)
	}
	if err := godotenv.Load(); err != nil {
		slog.Error("failed to load .env file", "error", err)
		os.Exit(1)
	}

	astraConfig := database.AstraConfig{
		Username: cfg.Database.Username,
		Path:     cfg.Database.Path,
		Token:    helpers.GetEnvOrDefault("ASTRA_TOKEN", ""),
	}

}
