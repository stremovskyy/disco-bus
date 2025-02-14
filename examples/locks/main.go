package main

import (
	"context"
	"log"
	"time"

	"github.com/stremovskyy/disco-bus"
)

func main() {
	bus := disco.NewDefaultRedisDiscoBus()
	ctx := context.Background()

	if err := bus.Start(ctx); err != nil {
		log.Fatalf("Failed to start bus: %v", err)
	}
	defer bus.Stop()

	// Try to acquire a lock
	lockKey := "my-critical-task"
	acquired, err := bus.AcquireLock(ctx, lockKey)
	if err != nil {
		log.Fatalf("Error acquiring lock: %v", err)
	}

	if acquired {
		log.Println("Lock acquired, performing critical task...")

		// Set expiration for the lock
		err = bus.Expire(ctx, lockKey, time.Second*30)
		if err != nil {
			log.Printf("Failed to set expiration: %v", err)
		}

		// Simulate some work
		time.Sleep(time.Second * 5)

		// Release the lock
		if err := bus.ReleaseLock(ctx, lockKey); err != nil {
			log.Printf("Failed to release lock: %v", err)
		}
		log.Println("Lock released")
	} else {
		log.Println("Could not acquire lock, task is already running")
	}
}
