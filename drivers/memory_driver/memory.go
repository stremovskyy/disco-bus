// file: drivers/memory_driver/memory.go
package memory_driver

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/stremovskyy/disco-bus/drivers"
)

var defaultLockExpiration = 5 * time.Minute

// memoryDriver is an in-memory implementation of the messaging bus driver.
type memoryDriver struct {
	// subscriptions stores one handler per topic.
	subsMutex     sync.RWMutex
	subscriptions map[string]func([]byte) error

	// locks stores distributed locks.
	locksMutex sync.Mutex
	locks      map[string]*lockInfo

	closeMutex sync.Mutex
	closed     bool
}

// lockInfo holds the expiration and timer for a lock.
type lockInfo struct {
	expiration time.Time
	timer      *time.Timer
}

// NewMemoryDriver creates a new in-memory driver instance.
func NewMemoryDriver() drivers.Driver {
	return &memoryDriver{
		subscriptions: make(map[string]func([]byte) error),
		locks:         make(map[string]*lockInfo),
	}
}

// Connect initializes the in-memory driver.
func (m *memoryDriver) Connect(ctx context.Context) error {
	m.closeMutex.Lock()
	defer m.closeMutex.Unlock()
	m.closed = false
	return nil
}

// Close clears subscriptions and stops any active lock timers.
func (m *memoryDriver) Close() error {
	m.closeMutex.Lock()
	defer m.closeMutex.Unlock()
	m.closed = true

	// Stop all active lock timers.
	m.locksMutex.Lock()
	for _, lock := range m.locks {
		if lock.timer != nil {
			lock.timer.Stop()
		}
	}
	m.locks = make(map[string]*lockInfo)
	m.locksMutex.Unlock()

	// Clear subscriptions.
	m.subsMutex.Lock()
	m.subscriptions = make(map[string]func([]byte) error)
	m.subsMutex.Unlock()

	return nil
}

// CreateTopic is a no-op for in-memory topics (they are created on demand).
func (m *memoryDriver) CreateTopic(ctx context.Context, topic string) error {
	if topic == "" {
		return errors.New("topic cannot be empty")
	}
	return nil
}

// Publish looks up the handler for the given topic and invokes it asynchronously.
// It returns the number of subscribers that received the message (0 or 1).
func (m *memoryDriver) Publish(ctx context.Context, topic string, message interface{}) (int64, error) {
	if topic == "" {
		return 0, errors.New("topic cannot be empty")
	}
	msgBytes, ok := message.([]byte)
	if !ok {
		return 0, errors.New("invalid message type; expected []byte")
	}

	// Retrieve the handler for the topic.
	m.subsMutex.RLock()
	handler, exists := m.subscriptions[topic]
	m.subsMutex.RUnlock()

	if !exists {
		return 0, nil // no subscribers
	}

	// Invoke the handler asynchronously.
	go func() {
		// In a real-world scenario, you might want to handle errors or context cancellation.
		_ = handler(msgBytes)
	}()

	return 1, nil
}

// Subscribe registers a handler for the specified topic.
func (m *memoryDriver) Subscribe(ctx context.Context, topic string, handler func([]byte) error) error {
	if topic == "" {
		return errors.New("topic cannot be empty")
	}
	if handler == nil {
		return errors.New("handler cannot be nil")
	}

	m.subsMutex.Lock()
	defer m.subsMutex.Unlock()
	m.subscriptions[topic] = handler
	return nil
}

// Unsubscribe removes the handler for the specified topic.
func (m *memoryDriver) Unsubscribe(topic string) error {
	if topic == "" {
		return errors.New("topic cannot be empty")
	}
	m.subsMutex.Lock()
	defer m.subsMutex.Unlock()
	delete(m.subscriptions, topic)
	return nil
}

// AcquireLock tries to acquire a distributed lock identified by lockName.
// If the lock is not already held or has expired, it sets a default expiration.
func (m *memoryDriver) AcquireLock(ctx context.Context, lockName string) (bool, error) {
	if lockName == "" {
		return false, errors.New("lock key cannot be empty")
	}

	m.locksMutex.Lock()
	defer m.locksMutex.Unlock()

	// Check if the lock exists and is still valid.
	if lock, exists := m.locks[lockName]; exists {
		if time.Now().Before(lock.expiration) {
			return false, nil
		}
		// Lock expired; clean it up.
		if lock.timer != nil {
			lock.timer.Stop()
		}
		delete(m.locks, lockName)
	}

	expiration := time.Now().Add(defaultLockExpiration)
	timer := time.AfterFunc(
		defaultLockExpiration, func() {
			m.locksMutex.Lock()
			delete(m.locks, lockName)
			m.locksMutex.Unlock()
		},
	)
	m.locks[lockName] = &lockInfo{
		expiration: expiration,
		timer:      timer,
	}
	return true, nil
}

// ReleaseLock releases a lock. Returns an error if the lock was not held.
func (m *memoryDriver) ReleaseLock(ctx context.Context, lockName string) error {
	if lockName == "" {
		return errors.New("lock key cannot be empty")
	}

	m.locksMutex.Lock()
	defer m.locksMutex.Unlock()
	lock, exists := m.locks[lockName]
	if !exists {
		return errors.New("failed to release lock: lock is not owned")
	}
	if lock.timer != nil {
		lock.timer.Stop()
	}
	delete(m.locks, lockName)
	return nil
}

// Expire resets the expiration for a key (lock) to the given duration.
func (m *memoryDriver) Expire(ctx context.Context, key string, duration time.Duration) error {
	if key == "" {
		return errors.New("key cannot be empty")
	}

	m.locksMutex.Lock()
	defer m.locksMutex.Unlock()
	lock, exists := m.locks[key]
	if !exists {
		// Key does not exist; nothing to expire.
		return nil
	}
	if lock.timer != nil {
		lock.timer.Stop()
	}
	expiration := time.Now().Add(duration)
	timer := time.AfterFunc(
		duration, func() {
			m.locksMutex.Lock()
			delete(m.locks, key)
			m.locksMutex.Unlock()
		},
	)
	lock.expiration = expiration
	lock.timer = timer
	return nil
}
