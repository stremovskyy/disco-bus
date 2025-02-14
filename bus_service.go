package disco

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/stremovskyy/disco-bus/drivers"
	"github.com/stremovskyy/disco-bus/drivers/memory_driver"
	"github.com/stremovskyy/disco-bus/drivers/redis_driver"
	"github.com/stremovskyy/disco-bus/monitoring"
)

type svc struct {
	state  atomic.Value
	driver drivers.Driver
}

// Error types for better error handling
var (
	ErrTopicEmpty   = errors.New("topic name cannot be empty")
	ErrNilMessage   = errors.New("message cannot be nil")
	ErrNilHandler   = errors.New("handler function cannot be nil")
	ErrNilDriver    = errors.New("driver cannot be nil")
	ErrNotConnected = errors.New("not connected to the message bus")
)

func NewDiscoBus(driver drivers.Driver) Bus {
	s := &svc{
		driver: driver,
	}

	return s
}

func NewDefaultRedisDiscoBus() Bus {
	return NewDiscoBus(redis_driver.DefaultRedisDriver())
}

func NewDefaultMemoryDiscoBus() Bus {
	return NewDiscoBus(memory_driver.NewMemoryDriver())
}

func (s *svc) PublishToTopic(ctx context.Context, topic string, message []byte) (int64, error) {
	if topic == "" {
		return 0, ErrTopicEmpty
	}
	if message == nil {
		return 0, ErrNilMessage
	}
	if s.driver == nil {
		return 0, ErrNilDriver
	}

	// Only add a timeout if ctx has no deadline.
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	start := time.Now()
	msgID, err := s.driver.Publish(ctx, topic, message)
	if err != nil {
		monitoring.MessagePublished.WithLabelValues(topic, "error").Inc()
		return 0, err
	}

	monitoring.MessagePublished.WithLabelValues(topic + "success").Inc()
	monitoring.ProcessingDuration.WithLabelValues(topic).Observe(time.Since(start).Seconds())

	return msgID, nil
}

func (s *svc) SubscribeHandler(ctx context.Context, topic string, handler func([]byte) error) error {
	if topic == "" {
		return ErrTopicEmpty
	}
	if handler == nil {
		return ErrNilHandler
	}
	if s.driver == nil {
		return ErrNilDriver
	}

	monitoring.ActiveSubscriptions.WithLabelValues(topic).Inc()

	// Wrap handler with recovery and metrics.
	safeHandler := func(msg []byte) error {
		start := time.Now()
		defer func() {
			if r := recover(); r != nil {
				monitoring.MessageProcessed.WithLabelValues(topic, "panic").Inc()
				log.Error().
					Interface("panic", r).
					Str("topic", topic).
					Msg("Recovered from panic in message handler")
			}
		}()

		err := handler(msg)
		duration := time.Since(start)
		status := "success"
		if err != nil {
			status = "error"
		}
		monitoring.MessageProcessed.WithLabelValues(topic, status).Inc()
		monitoring.ProcessingDuration.WithLabelValues(topic).Observe(duration.Seconds())
		return err
	}

	return s.driver.Subscribe(ctx, topic, safeHandler)
}

func (s *svc) UnsubscribeFromTopic(topicName string) {
	if err := s.driver.Unsubscribe(topicName); err != nil {
		log.Error().Err(err).Str("topic", topicName).Msg("Failed to unsubscribe from topic")
	}
	monitoring.ActiveSubscriptions.WithLabelValues(topicName).Dec()
}

func (s *svc) Start(ctx context.Context) error {
	return s.driver.Connect(ctx)
}

func (s *svc) Stop() error {
	return s.driver.Close()
}

func (s *svc) AcquireLock(ctx context.Context, lockKey string) (bool, error) {
	if lockKey == "" {
		return false, errors.New("lock key cannot be empty")
	}
	if s.driver == nil {
		return false, ErrNilDriver
	}

	// Add timeout if context doesn't have one
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	acquired, err := s.driver.AcquireLock(ctx, lockKey)
	if err != nil {
		monitoring.LockOperations.WithLabelValues("acquire", "error").Inc()
		return false, err
	}

	if acquired {
		monitoring.LockOperations.WithLabelValues("acquire", "success").Inc()
	} else {
		monitoring.LockOperations.WithLabelValues("acquire", "failed").Inc()
	}

	return acquired, nil
}

func (s *svc) ReleaseLock(ctx context.Context, lockKey string) error {
	if lockKey == "" {
		return errors.New("lock key cannot be empty")
	}

	err := s.driver.ReleaseLock(ctx, lockKey)
	if err != nil {
		monitoring.LockOperations.WithLabelValues("release", "error").Inc()
		return err
	}

	monitoring.LockOperations.WithLabelValues("release", "success").Inc()
	return nil
}

func (s *svc) Expire(ctx context.Context, lockKey string, duration time.Duration) error {
	return s.driver.Expire(ctx, lockKey, duration)
}
