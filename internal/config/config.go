package config

import "fmt"

// Config holds all application configuration
type Config struct {
	// HTTP server settings
	Host string
	Port int

	// Valkey/Redis connection
	ValkeyURL      string
	ValkeyPassword string
	ValkeyDB       int

	// UI settings
	OpenBrowser bool
}

// New creates a new Config with default values
func New() *Config {
	return &Config{
		Host:      "localhost",
		Port:      8080,
		ValkeyURL: "localhost:6379",
		ValkeyDB:  0,
	}
}

// Addr returns the HTTP server address
func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
