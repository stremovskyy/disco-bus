package disco

import (
	"context"
	"time"
)

// Bus an interface for a messaging bus
type Bus interface {
	Start(ctx context.Context) error
	Stop() error
	PubSub() PubSub
	Lock() Lock
}

type PubSub interface {
	Publish(ctx context.Context, topic string, message []byte) (int64, error)
	SubscribeHandler(ctx context.Context, topic string, handler func([]byte) error) error
	Unsubscribe(topicName string)
}

type Lock interface {
	Acquire(ctx context.Context, task string) (bool, error)
	Release(ctx context.Context, task string) error
	Expire(ctx context.Context, key string, duration time.Duration) error
	Refresh(ctx context.Context, key string, duration time.Duration) error
}
