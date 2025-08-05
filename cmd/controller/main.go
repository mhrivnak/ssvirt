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

	log.Printf("Starting controller manager in namespace %s", cfg.Kubernetes.Namespace)
	// TODO: Initialize controller manager
}
