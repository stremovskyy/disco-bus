package drivers

import (
	"context"
	"time"
)

type Driver interface {
	// Connect to driver
	Connect(context.Context) error

	// Close closes the connection to the driver
	Close() error

	// CreateTopic creates a topic
	CreateTopic(ctx context.Context, topic string) error

	// Publish publishes the message
	Publish(ctx context.Context, topic string, message interface{}) (int64, error)

	// Subscribe subscribes to the topic
	Subscribe(ctx context.Context, topic string, handler func([]byte) error) error

	// Unsubscribe unsubscribes from the topic
	Unsubscribe(topic string) error

	// Acquire acquires a distributed lock
	Acquire(ctx context.Context, lockName string) (bool, error)

	// ReleaseLock releases a distributed lock
	Release(ctx context.Context, lockName string) error

	// Expire sets a key expiration
	Expire(ctx context.Context, key string, duration time.Duration) error

	// Refresh refreshes a key expiration
	Refresh(ctx context.Context, key string, duration time.Duration) error
}
