package disco

import (
	"context"
	"time"
)

// Bus an interface for a messaging bus
type Bus interface {
	Start(ctx context.Context) error
	Stop() error
	PublishToTopic(ctx context.Context, topic string, message []byte) (int64, error)
	SubscribeHandler(ctx context.Context, topic string, handler func([]byte) error) error
	UnsubscribeFromTopic(topicName string)
	AcquireLock(ctx context.Context, task string) (bool, error)
	ReleaseLock(ctx context.Context, task string) error
	Expire(ctx context.Context, key string, duration time.Duration) error
}
