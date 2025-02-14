package redis_driver

import (
	"compress/gzip"
	"time"
)

type Config struct {
	Host            string
	Port            string
	DB              int
	Password        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	PoolSize        int
}

func DefaultRedisConfig() Config {
	return Config{
		Host:            "localhost",
		Port:            "6379",
		DB:              0,
		Password:        "",
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		PoolSize:        10,
	}
}

type Options struct {
	Writer          Config
	Reader          *Config
	TopicPrefix     string
	MaxRetries      int
	MaxRedisWorkers int
	MaxMessageSize  int
	LockExpiration  time.Duration
	BufferSize      int
	CompressionLvl  int
}

func DefaultOptions() Options {
	return Options{
		Writer:          DefaultRedisConfig(),
		TopicPrefix:     "disco",
		MaxRetries:      3,
		MaxRedisWorkers: 5,
		MaxMessageSize:  1024 * 1024 * 10,
		LockExpiration:  5 * time.Minute,
		BufferSize:      1024 * 1024 * 10,
		CompressionLvl:  gzip.BestCompression,
	}
}

func (o Config) DSN() string {
	return o.Host + ":" + o.Port
}
