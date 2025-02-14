# Disco Bus

_Discovery Bus (Disco Bus)_ is a high-performance, distributed messaging bus written in Go. It simplifies the implementation of pub/sub systems and distributed locks with built-in error handling, metrics, and pluggable drivers (currently featuring a robust Redis driver). Whether you're building microservices, event-driven systems, or distributed applications, Disco Bus offers an extensible and production-ready messaging solution.

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [Usage](#usage)
  - [Starting the Bus](#starting-the-bus)
  - [Publishing Messages](#publishing-messages)
  - [Subscribing to Topics](#subscribing-to-topics)
  - [Distributed Locks](#distributed-locks)
- [Architecture](#architecture)
- [Examples](#examples)
- [Testing](#testing)
- [Contributing](#contributing)
- [License](#license)
- [Acknowledgements](#acknowledgements)

## Features

- **Publish/Subscribe Messaging:** Easily publish messages to topics and subscribe with custom handlers.
- **Distributed Locking:** Acquire, release, and manage distributed locks for critical sections.
- **Pluggable Drivers:** Comes with a Redis driver supporting features like gzip compression and multiple worker routines.
- **Built-in Metrics:** Integrated with Prometheus to track message publishing, processing durations, active subscriptions, and lock operations.
- **Robust Error Handling:** Provides detailed error types, panic recovery in handlers, and context-based timeouts.
- **Extensible Architecture:** Easily extend or replace drivers, add middleware, or integrate additional monitoring solutions.

## Installation

Ensure you have Go (1.18+) installed, then install Disco Bus with:

```bash
go get github.com/stremovskyy/disco-bus
```

Or, clone the repository:

```bash
git clone https://github.com/stremovskyy/disco-bus.git
cd disco-bus
```

## Usage

Disco Bus is designed for ease of use. Below are some common usage patterns.

### Starting the Bus

Initialize and start the bus using the default Redis driver:

```go
package main

import (
	"context"
	"log"

	"github.com/stremovskyy/disco-bus"
)

func main() {
	// Create a new Disco Bus instance with the default Redis driver
	bus := disco.NewDefaultDiscoBus()
	ctx := context.Background()

	// Start the bus
	if err := bus.Start(ctx); err != nil {
		log.Fatalf("Failed to start bus: %v", err)
	}
	defer bus.Stop()

	// The bus is now ready for publishing and subscribing
}
```

### Publishing Messages

Publish a message to a topic with a context-based timeout:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

msgID, err := bus.PublishToTopic(ctx, "orders", []byte(`{"order_id": "123"}`))
if err != nil {
	log.Printf("Failed to publish message: %v", err)
} else {
	log.Printf("Published message with ID: %d", msgID)
}
```

### Subscribing to Topics

Subscribe to a topic with a custom message handler. The handler is wrapped with error recovery and metrics instrumentation:

```go
err := bus.SubscribeHandler(ctx, "orders", func(msg []byte) error {
	// Process incoming message
	log.Printf("Received order: %s", string(msg))
	return nil
})
if err != nil {
	log.Fatalf("Failed to subscribe: %v", err)
}
```

### Distributed Locks

Manage critical sections by acquiring and releasing distributed locks:

```go
lockKey := "my-critical-task"
acquired, err := bus.AcquireLock(ctx, lockKey)
if err != nil {
	log.Fatalf("Error acquiring lock: %v", err)
}

if acquired {
	log.Println("Lock acquired, performing critical task...")

	// Optionally set a new expiration for the lock
	if err := bus.Expire(ctx, lockKey, 30*time.Second); err != nil {
		log.Printf("Failed to set lock expiration: %v", err)
	}

	// Execute critical operations here...

	// Release the lock after completion
	if err := bus.ReleaseLock(ctx, lockKey); err != nil {
		log.Printf("Failed to release lock: %v", err)
	}
	log.Println("Lock released")
} else {
	log.Println("Could not acquire lock; task is already running")
}
```

## Architecture

Disco Bus is organized into several key packages:

- **`disco`**:  
  Contains the main `Bus` interface and service implementations. It abstracts common operations like publishing, subscribing, and lock management.

- **`drivers`**:  
  Defines the pluggable driver interface. The default implementation using Redis is found under `drivers/redis_driver`, which includes:
    - **Connection Management:** Establishes and closes connections to Redis.
    - **Message Compression:** Uses gzip compression to optimize message delivery.
    - **Worker Routines:** Handles message subscriptions and processing concurrently.

- **`monitoring`**:  
  Integrates Prometheus metrics to monitor:
    - Published and processed messages.
    - Processing durations.
    - Lock operations.
    - Active subscriptions.

- **`extensions`**:  
  Provides utilities such as a resettable gzip reader for efficient message decompression.

- **`examples`**:  
  Contains sample applications demonstrating various use cases:
    - **`contexter`**: Scans directories and processes Go files.
    - **`error_handling`**: Demonstrates robust error handling and exposes a Prometheus metrics endpoint.
    - **`locks`**: Illustrates distributed lock acquisition, expiration, and release.
    - **`pub-sub`**: A simple publish-subscribe example.

## Examples

The `examples` directory provides practical demonstrations of Disco Bus in action. To run an example:

1. Navigate to the desired example directory, for instance:

   ```bash
   cd examples/pub-sub
   ```

2. Run the example:

   ```bash
   go run main.go
   ```

Each example showcases a different aspect of the system—from context processing and error handling to pub/sub messaging and distributed locks.

## Testing

Disco Bus comes with comprehensive unit tests to ensure reliability. To run all tests:

```bash
go test ./...
```

The tests cover key functionalities including:
- Message publishing and subscription.
- Distributed lock operations.
- Driver interactions using mocked implementations.

## Contributing

Contributions are highly welcome! To contribute:

1. **Fork** the repository.
2. Create a new branch:
   ```bash
   git checkout -b feature/my-feature
   ```
3. **Commit** your changes:
   ```bash
   git commit -am 'Add new feature'
   ```
4. **Push** to your branch:
   ```bash
   git push origin feature/my-feature
   ```
5. Open a **Pull Request**.

For significant changes, please open an issue first to discuss your proposal.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Acknowledgements

- **[Redis](https://redis.io/):** In-memory data structure store.
- **[Prometheus](https://prometheus.io/):** Monitoring and metrics.
- **[zerolog](https://github.com/rs/zerolog):** Fast and efficient logging.
- **[go-redis](https://github.com/redis/go-redis):** Redis client for Go.
- **[stretchr/testify](https://github.com/stretchr/testify):** Testing framework.

---
