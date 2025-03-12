package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/stremovskyy/disco-bus"
)

func main() {
	// Start Prometheus metrics endpoint
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Fatal(http.ListenAndServe(":2112", nil))
	}()

	bus := disco.NewDefaultRedisDiscoBus()
	ctx := context.Background()

	if err := bus.Start(ctx); err != nil {
		log.Fatalf("Failed to start bus: %v", err)
	}
	defer bus.Stop()

	// Subscribe with error handling
	err := bus.PubSub().SubscribeHandler(
		ctx, "orders", func(msg []byte) error {
			// Simulate processing
			if len(msg) == 0 {
				return disco.ErrNilMessage
			}

			log.Printf("Processing order: %s", string(msg))
			return nil
		},
	)
	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}

	// Publish messages with context timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	msgID, err := bus.PubSub().Publish(ctx, "orders", []byte(`{"order_id": "123"}`))
	if err != nil {
		log.Printf("Failed to publish: %v", err)
	} else {
		log.Printf("Published order message with ID: %d", msgID)
	}

	// Keep the application running
	select {}
}
