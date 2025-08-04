package main

import (
	"log"

	"github.com/mhrivnak/ssvirt/pkg/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Starting API server on port %d", cfg.API.Port)
	// TODO: Initialize API server
}