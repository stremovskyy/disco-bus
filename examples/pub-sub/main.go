package main

import (
	"context"
	"log"
	"time"

	"github.com/stremovskyy/disco-bus"
)

func main() {
	// Create a new disco bus instance with default Redis driver
	bus := disco.NewDefaultRedisDiscoBus()

	// Start the bus
	ctx := context.Background()
	if err := bus.Start(ctx); err != nil {
		log.Fatalf("Failed to start bus: %v", err)
	}
	defer bus.Stop()

	// Subscribe to a topic
	err := bus.SubscribeHandler(
		ctx, "notifications", func(msg []byte) error {
			log.Printf("Received message: %s", string(msg))
			return nil
		},
	)
	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}

	// Publish a message
	msgID, err := bus.PublishToTopic(ctx, "notifications", []byte("Hello, World!"))
	if err != nil {
		log.Fatalf("Failed to publish: %v", err)
	}
	log.Printf("Published message with ID: %d", msgID)

	// Keep the program running to receive messages
	time.Sleep(time.Second * 5)
}
