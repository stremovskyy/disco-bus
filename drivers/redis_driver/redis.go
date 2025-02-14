package redis_driver

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/stremovskyy/disco-bus/drivers"
)

var (
	gzipWriterPool sync.Pool
)

var bufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, DefaultOptions().BufferSize))
	},
}

type redisDriver struct {
	sync.RWMutex

	workers            []*Worker
	options            *Options
	reader             *redis.Client
	writer             *redis.Client
	subsStarted        bool
	subscriptionsIndex map[string]int // topic -> worker index
	zipBuffer          *bytes.Buffer
	zipBufferMutex     *sync.Mutex
	ctx                context.Context
	cancelFunc         context.CancelFunc
	wg                 sync.WaitGroup
}

func NewRedisDriver(options *Options) drivers.Driver {
	gzipWriterPool = sync.Pool{
		New: func() interface{} {
			writer, err := gzip.NewWriterLevel(nil, options.CompressionLvl)
			if err != nil {
				return nil
			}

			return writer
		},
	}

	return &redisDriver{
		options:            options,
		subscriptionsIndex: make(map[string]int),
		zipBuffer:          bytes.NewBuffer(make([]byte, 0, options.BufferSize)),
		zipBufferMutex:     &sync.Mutex{},
	}
}

func DefaultRedisDriver() drivers.Driver {
	options := DefaultOptions()

	gzipWriterPool = sync.Pool{
		New: func() interface{} {
			writer, err := gzip.NewWriterLevel(nil, options.CompressionLvl)
			if err != nil {
				return nil
			}

			return writer
		},
	}

	return &redisDriver{
		options:            &options,
		subscriptionsIndex: make(map[string]int),
		zipBuffer:          bytes.NewBuffer(make([]byte, 0, options.BufferSize)),
		zipBufferMutex:     &sync.Mutex{},
	}
}

func (d *redisDriver) Connect(ctx context.Context) error {
	d.ctx, d.cancelFunc = context.WithCancel(context.Background())

	d.writer = redis.NewClient(
		&redis.Options{
			MaxRetries:      d.options.MaxRetries,
			Addr:            d.options.Writer.DSN(),
			Password:        d.options.Writer.Password,
			DB:              d.options.Writer.DB,
			MaxIdleConns:    d.options.Writer.MaxIdleConns,
			ConnMaxLifetime: d.options.Writer.ConnMaxLifetime,
			PoolSize:        d.options.Writer.PoolSize,
		},
	)

	if d.options.Reader == nil {
		d.reader = d.writer
	} else {
		d.reader = redis.NewClient(
			&redis.Options{
				MaxRetries:      d.options.MaxRetries,
				Addr:            d.options.Reader.DSN(),
				Password:        d.options.Reader.Password,
				DB:              d.options.Reader.DB,
				MaxIdleConns:    d.options.Reader.MaxIdleConns,
				ConnMaxLifetime: d.options.Reader.ConnMaxLifetime,
				PoolSize:        d.options.Reader.PoolSize,
			},
		)
	}

	result, err := d.reader.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to redis: %w", err)
	}

	log.Trace().Msgf("connected to redis reader, result: %s", result)

	result, err = d.writer.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to redis: %w", err)
	}

	log.Trace().Msgf("connected to redis writer, result: %s", result)

	d.workers = make([]*Worker, d.options.MaxRedisWorkers)
	for i := 0; i < d.options.MaxRedisWorkers; i++ {
		d.workers[i] = NewWorker(i, d.reader, d.options.MaxMessageSize)
		d.wg.Add(1)
		go func(worker *Worker) {
			defer d.wg.Done()
			worker.Start(d.ctx) // Pass driver's context to worker
		}(d.workers[i])
	}

	log.Trace().Int("redis_workers", d.options.MaxRedisWorkers).Msgf("initialized %d REDIS workers", d.options.MaxRedisWorkers)

	return nil
}

func (d *redisDriver) CreateTopic(ctx context.Context, topic string) error {
	result, err := d.writer.Publish(ctx, topic, "").Result()
	if err != nil {
		return fmt.Errorf("failed to create topic %s: %w", topic, err)
	}

	log.Trace().Msgf("created topic %s, result: %d", topic, result)

	return nil
}

func (d *redisDriver) Close() error {
	if d.cancelFunc != nil {
		d.cancelFunc()
	}

	d.wg.Wait()

	err := d.reader.Close()
	if err != nil {
		return fmt.Errorf("failed to close redis reader: %w", err)
	}
	err = d.writer.Close()
	if err != nil {
		return fmt.Errorf("failed to close redis writer: %w", err)
	}

	log.Trace().Msg("closed redis connections")
	return nil
}

func (d *redisDriver) Publish(ctx context.Context, topic string, message interface{}) (int64, error) {
	msgBytes, ok := message.([]byte)
	if !ok {
		return 0, fmt.Errorf("invalid message type; expected []byte")
	}

	data, err := d.gzip(msgBytes)
	if err != nil {
		return 0, fmt.Errorf("failed to compress message for topic %s: %w", topic, err)
	}

	clientsReceived, err := d.writer.Publish(ctx, topic, data).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to publish message to topic %s: %w", topic, err)
	}

	log.Trace().Msgf("Published compressed message to topic %s, clients received: %d", topic, clientsReceived)
	return clientsReceived, nil
}

func (d *redisDriver) Subscribe(ctx context.Context, topic string, handler func([]byte) error) error {
	worker := d.selectWorkerForSubscription()

	err := worker.AddSubscription(ctx, topic, handler)
	if err != nil {
		return fmt.Errorf("failed to subscribe to topic %s: %w", topic, err)
	}

	d.Lock()
	defer d.Unlock()

	d.subscriptionsIndex[topic] = worker.OwnIndex

	log.Trace().Msgf("[Redis Messaging Driver] subscribed to topic %s", topic)

	return nil
}

func (d *redisDriver) Unsubscribe(topic string) error {
	d.Lock()
	defer d.Unlock()

	workerIndex, exists := d.subscriptionsIndex[topic]
	if !exists {
		return nil
	}

	return d.workers[workerIndex].RemoveSubscription(topic)
}

func (d *redisDriver) AcquireLock(ctx context.Context, lockName string) (bool, error) {
	result, err := d.writer.SetNX(ctx, lockName, "lock", d.options.LockExpiration).Result()
	if err != nil {
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}
	return result, nil
}

func (d *redisDriver) ReleaseLock(ctx context.Context, lockName string) error {
	script := redis.NewScript(
		`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		else
			return 0
		end
	`,
	)
	result, err := script.Run(ctx, d.writer, []string{lockName}, "lock").Result()
	if err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}
	if result.(int64) == 0 {
		return fmt.Errorf("failed to release lock: lock is not owned by current client")
	}
	return nil
}
func (d *redisDriver) selectWorkerForSubscription() *Worker {
	if len(d.workers) == 0 {
		time.Sleep(time.Millisecond * 100)
		for tries := 0; tries <= 10; tries++ {
			if len(d.workers) == 0 {
				time.Sleep(time.Millisecond * 100)
				log.Trace().Msg("no workers available, waiting...")
			} else {
				break
			}
		}
	}

	selectedWorker := d.workers[0]
	minSubs := len(selectedWorker.handlers)

	for _, worker := range d.workers {
		if workerSubs := len(worker.handlers); workerSubs < minSubs {
			minSubs = workerSubs
			selectedWorker = worker
		}
	}

	return selectedWorker
}

func (d *redisDriver) Expire(ctx context.Context, key string, duration time.Duration) error {
	return d.writer.Expire(ctx, key, duration).Err()
}

func (d *redisDriver) gzip(data []byte) ([]byte, error) {
	// Get a buffer from the pool and reset it.
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	// Get a gzip writer from the pool (see below for the writer pool)
	gw, ok := gzipWriterPool.Get().(*gzip.Writer)
	if !ok || gw == nil {
		var err error
		gw, err = gzip.NewWriterLevel(buf, d.options.CompressionLvl)
		if err != nil {
			return nil, err
		}
	} else {
		gw.Reset(buf)
	}

	_, err := gw.Write(data)
	if err != nil {
		return nil, err
	}
	if err = gw.Close(); err != nil {
		return nil, err
	}
	gzipWriterPool.Put(gw)
	return buf.Bytes(), nil
}
