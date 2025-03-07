package main

import (
	"embed"
	"interview/internal/api"
	"interview/internal/config"
	"interview/internal/repo"
	"log"

	"github.com/joho/godotenv"
)

//go:embed templates
var templateFS embed.FS

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	// Initialize database
	db, err := repo.InitDatabase(*cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	api.InitAPI(db, templateFS, *cfg)
}
