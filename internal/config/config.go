package config

import (
	"fmt"
	"os"
)

// Config holds all configuration values for the application.
type Config struct {
	// DBHost is the hostname of the MySQL database server
	DBHost string
	// DBPort is the port number of the MySQL database server
	DBPort string
	// DBUser is the username for connecting to the MySQL database
	DBUser string
	// DBPassword is the password for connecting to the MySQL database
	DBPassword string
	// DBName is the name of the MySQL database to use
	DBName string
	// SessionSecret is used to encrypt session data and generate CSRF tokens
	SessionSecret string
	// SessionName is the name of the session cookie
	SessionName string
	// APIPort is the port number on which the HTTP server will listen
	APIPort string
}

// Load reads configuration from environment variables and validates them.
// Returns an error if any required environment variable is missing.
func Load() (*Config, error) {
	cfg := &Config{
		DBHost:        os.Getenv("DB_HOST"),
		DBPort:        os.Getenv("DB_PORT"),
		DBUser:        os.Getenv("DB_USER"),
		DBPassword:    os.Getenv("DB_PASSWORD"),
		DBName:        os.Getenv("DB_DATABASE"),
		SessionSecret: os.Getenv("SESSION_SECRET"),
		SessionName:   os.Getenv("SESSION_NAME"),
		APIPort:       os.Getenv("API_PORT"),
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// validate checks if all required configuration values are present.
func (c *Config) validate() error {
	if c.DBHost == "" {
		return fmt.Errorf("DB_HOST is required")
	}
	if c.DBPort == "" {
		return fmt.Errorf("DB_PORT is required")
	}
	if c.DBUser == "" {
		return fmt.Errorf("DB_USER is required")
	}
	if c.DBPassword == "" {
		return fmt.Errorf("DB_PASSWORD is required")
	}
	if c.DBName == "" {
		return fmt.Errorf("DB_DATABASE is required")
	}
	if c.SessionSecret == "" {
		return fmt.Errorf("SESSION_SECRET is required")
	}
	if c.SessionName == "" {
		return fmt.Errorf("SESSION_NAME is required")
	}
	if c.APIPort == "" {
		return fmt.Errorf("API_PORT is required")
	}
	return nil
}
