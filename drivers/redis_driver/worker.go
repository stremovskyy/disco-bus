package redis_driver

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/stremovskyy/disco-bus/extensions"
)

var (
	gzipReaderPool = sync.Pool{
		New: func() interface{} {
			return new(extensions.ResettableGzipReader)
		},
	}
)

type Worker struct {
	pubsub            *redis.PubSub
	handlers          map[string]func(m *redis.Message) error
	OwnIndex          int
	mu                sync.RWMutex
	gunzipBuffer      *bytes.Buffer
	gunzipBufferMutex *sync.Mutex
}

func NewWorker(ownIndex int, client *redis.Client, _ int) *Worker {
	return &Worker{
		handlers: make(map[string]func(m *redis.Message) error),
		OwnIndex: ownIndex,
		pubsub:   client.Subscribe(context.Background()),
	}
}

func (w *Worker) Start(ctx context.Context) {
	defer w.pubsub.Close()

	for {
		select {
		case message, ok := <-w.pubsub.Channel():
			if !ok {
				return // Channel was closed, exit the worker
			}
			topic := message.Channel

			// Obtain the handler safely.
			w.mu.RLock()
			handler, exists := w.handlers[topic]
			w.mu.RUnlock()
			if !exists {
				log.Error().Msgf("received a message for an unknown topic: %s", topic)
				continue
			}

			// Process each message in its own goroutine (if ordering isn’t critical).
			go func(msg *redis.Message) {
				if err := handler(msg); err != nil {
					log.Error().Err(err).Msgf("failed to handle message for topic %s", topic)
				}
			}(message)

		case <-ctx.Done():
			// Safely grab the list of topics before unsubscribing.
			w.mu.RLock()
			topics := make([]string, 0, len(w.handlers))
			for topic := range w.handlers {
				topics = append(topics, topic)
			}
			w.mu.RUnlock()

			if len(topics) > 0 {
				if err := w.pubsub.Unsubscribe(ctx, topics...); err != nil {
					log.Error().Err(err).Msg("failed to unsubscribe topics")
				}
			}
			if err := w.pubsub.Close(); err != nil {
				log.Error().Msgf("failed to close pubsub connection: %s", err)
			}
			return
		}
	}
}

func (w *Worker) AddSubscription(ctx context.Context, topic string, handler func([]byte) error) error {
	err := w.pubsub.Subscribe(ctx, topic)
	if err != nil {
		return fmt.Errorf("failed to subscribe to topic %s: %w", topic, err)
	}

	w.mu.Lock()
	w.handlers[topic] = messageHandlerDecoder(handler)
	w.mu.Unlock()

	return nil
}

func (w *Worker) RemoveSubscription(topic string) error {
	err := w.pubsub.Unsubscribe(context.Background(), topic)

	w.mu.Lock()
	delete(w.handlers, topic)
	w.mu.Unlock()

	return err
}

func messageHandlerDecoder(handler func([]byte) error) func(m *redis.Message) error {
	return func(m *redis.Message) error {
		if m == nil {
			return fmt.Errorf("redis message is nil")
		}

		var JsonString []byte
		var err error

		// check if message is gzip compressed
		if m.Payload[0] != 0x1f || m.Payload[1] != 0x8b {
			log.Trace().Msgf("message is not gzip compressed")
			JsonString = []byte(m.Payload)
		} else {
			// Decompress gzip message
			JsonString, err = gunzip([]byte(m.Payload))
			if err != nil {
				return fmt.Errorf("failed to decompress gzip message: %w", err)
			}
		}

		err = handler(JsonString)
		if err != nil {
			return fmt.Errorf("failed to handle message: %w", err)
		}

		return nil
	}
}

func gunzip(data []byte) ([]byte, error) {
	gr := gzipReaderPool.Get().(*extensions.ResettableGzipReader)
	err := gr.Reset(data)
	if err != nil {
		return nil, err
	}
	defer gzipReaderPool.Put(gr)

	var buf bytes.Buffer
	_, err = buf.ReadFrom(gr)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
