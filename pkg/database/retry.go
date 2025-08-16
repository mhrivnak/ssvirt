package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mhrivnak/ssvirt/pkg/config"
)

// RetryConfig contains configuration for database connection retries
type RetryConfig struct {
	MaxAttempts     int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffMultiple float64
}

// DefaultRetryConfig returns sensible defaults for database connection retries
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:     30,               // Try for up to 30 attempts
		InitialDelay:    2 * time.Second,  // Start with 2 second delays
		MaxDelay:        30 * time.Second, // Cap delays at 30 seconds
		BackoffMultiple: 1.5,              // Increase delay by 50% each time
	}
}

// RetryConfigFromConfig creates a RetryConfig from the application configuration
func RetryConfigFromConfig(cfg *config.Config) RetryConfig {
	return RetryConfig{
		MaxAttempts:     cfg.Database.Retry.MaxAttempts,
		InitialDelay:    cfg.Database.Retry.InitialDelay,
		MaxDelay:        cfg.Database.Retry.MaxDelay,
		BackoffMultiple: cfg.Database.Retry.BackoffMultiple,
	}
}

// NewConnectionWithRetry attempts to connect to the database with exponential backoff retry logic
// This function will block until either a successful connection is established or the context is cancelled
func NewConnectionWithRetry(ctx context.Context, cfg *config.Config, retryConfig RetryConfig) (*DB, error) {
	var lastErr error
	delay := retryConfig.InitialDelay

	log.Printf("Attempting to connect to database with retry logic (max attempts: %d)", retryConfig.MaxAttempts)

	for attempt := 1; attempt <= retryConfig.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("database connection cancelled: %w", ctx.Err())
		default:
		}

		log.Printf("Database connection attempt %d/%d", attempt, retryConfig.MaxAttempts)

		db, err := NewConnection(cfg)
		if err == nil {
			log.Printf("Database connection established successfully on attempt %d", attempt)
			return db, nil
		}

		lastErr = err
		log.Printf("Database connection attempt %d failed: %v", attempt, err)

		// Don't wait after the final attempt
		if attempt == retryConfig.MaxAttempts {
			break
		}

		log.Printf("Waiting %v before next database connection attempt...", delay)

		// Use a timer so we can respect context cancellation during the delay
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, fmt.Errorf("database connection cancelled during retry delay: %w", ctx.Err())
		case <-timer.C:
			// Continue to next attempt
		}

		// Calculate next delay with exponential backoff
		delay = time.Duration(float64(delay) * retryConfig.BackoffMultiple)
		if delay > retryConfig.MaxDelay {
			delay = retryConfig.MaxDelay
		}
	}

	return nil, fmt.Errorf("failed to connect to database after %d attempts, last error: %w",
		retryConfig.MaxAttempts, lastErr)
}
