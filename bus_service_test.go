package disco

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockDriver is a mock implementation of the Driver interface
type MockDriver struct {
	mock.Mock
}

func (m *MockDriver) Connect(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockDriver) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDriver) CreateTopic(ctx context.Context, topic string) error {
	args := m.Called(ctx, topic)
	return args.Error(0)
}

func (m *MockDriver) Publish(ctx context.Context, topic string, message interface{}) (int64, error) {
	args := m.Called(ctx, topic, message)
	return int64(args.Int(0)), args.Error(1)
}

func (m *MockDriver) Subscribe(ctx context.Context, topic string, handler func([]byte) error) error {
	args := m.Called(ctx, topic, handler)
	return args.Error(0)
}

func (m *MockDriver) Unsubscribe(topic string) error {
	args := m.Called(topic)
	return args.Error(0)
}

func (m *MockDriver) Acquire(ctx context.Context, lockName string) (bool, error) {
	args := m.Called(ctx, lockName)
	return args.Bool(0), args.Error(1)
}

func (m *MockDriver) Release(ctx context.Context, lockName string) error {
	args := m.Called(ctx, lockName)
	return args.Error(0)
}

func (m *MockDriver) Expire(ctx context.Context, key string, duration time.Duration) error {
	args := m.Called(ctx, key, duration)
	return args.Error(0)
}

func (m *MockDriver) Refresh(ctx context.Context, key string, duration time.Duration) error {
	args := m.Called(ctx, key, duration)
	return args.Error(0)
}

func TestPublishToTopic(t *testing.T) {
	mockDriver := new(MockDriver)
	svc := NewDiscoBus(mockDriver)

	tests := []struct {
		name        string
		topic       string
		message     []byte
		setupMock   func()
		wantMsgID   int64
		wantErr     bool
		expectedErr error
	}{
		{
			name:    "successful publish",
			topic:   "test-topic",
			message: []byte("test message"),
			setupMock: func() {
				mockDriver.On("Publish", mock.Anything, "test-topic", []byte("test message")).
					Return(1, nil)
			},
			wantMsgID: 1,
			wantErr:   false,
		},
		{
			name:    "empty topic",
			topic:   "",
			message: []byte("test message"),
			setupMock: func() {
				// No mock setup needed as it should fail before calling driver
			},
			wantMsgID:   0,
			wantErr:     true,
			expectedErr: ErrTopicEmpty,
		},
		{
			name:    "nil message",
			topic:   "test-topic",
			message: nil,
			setupMock: func() {
				// No mock setup needed as it should fail before calling driver
			},
			wantMsgID:   0,
			wantErr:     true,
			expectedErr: ErrNilMessage,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				if tt.setupMock != nil {
					tt.setupMock()
				}

				msgID, err := svc.PubSub().Publish(context.Background(), tt.topic, tt.message)

				if tt.wantErr {
					assert.Error(t, err)
					if tt.expectedErr != nil {
						assert.Equal(t, tt.expectedErr, err)
					}
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tt.wantMsgID, msgID)
				}

				mockDriver.AssertExpectations(t)
			},
		)
	}
}

func TestAcquire(t *testing.T) {
	mockDriver := new(MockDriver)
	svc := NewDiscoBus(mockDriver)

	tests := []struct {
		name        string
		lockKey     string
		setupMock   func()
		want        bool
		wantErr     bool
		expectedErr error
	}{
		{
			name:    "successful lock acquisition",
			lockKey: "test-lock",
			setupMock: func() {
				mockDriver.On("Acquire", mock.Anything, "test-lock").
					Return(true, nil)
			},
			want:    true,
			wantErr: false,
		},
		{
			name:    "empty lock key",
			lockKey: "",
			setupMock: func() {
				// No mock setup needed as it should fail before calling driver
			},
			want:        false,
			wantErr:     true,
			expectedErr: errors.New("lock key cannot be empty"),
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				if tt.setupMock != nil {
					tt.setupMock()
				}

				got, err := svc.Lock().Acquire(context.Background(), tt.lockKey)

				if tt.wantErr {
					assert.Error(t, err)
					if tt.expectedErr != nil {
						assert.Equal(t, tt.expectedErr.Error(), err.Error())
					}
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tt.want, got)
				}

				mockDriver.AssertExpectations(t)
			},
		)
	}
}

func TestHighLoadPublishing(t *testing.T) {
	mockDriver := new(MockDriver)
	svc := NewDiscoBus(mockDriver)
	ctx := context.Background()
	topic := "highload-topic"
	message := []byte("load test message")

	// Set up the mock to simulate a fast, successful publish.
	mockDriver.On("Publish", mock.Anything, topic, message).
		Return(1, nil).Maybe() // .Maybe() allows multiple calls

	const numMessages = 1000
	errs := make(chan error, numMessages)
	for i := 0; i < numMessages; i++ {
		go func() {
			if _, err := svc.PubSub().Publish(ctx, topic, message); err != nil {
				errs <- err
			} else {
				errs <- nil
			}
		}()
	}

	for i := 0; i < numMessages; i++ {
		err := <-errs
		assert.NoError(t, err)
	}
	mockDriver.AssertExpectations(t)
}
