package telemetry

import (
	"strings"
	"time"

	"github.com/gomodule/redigo/redis"
)

// NewRedisPool creates a redis pool for telemetry work queues.
func NewRedisPool(redisURL string) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			if strings.Contains(redisURL, "://") {
				return redis.DialURL(redisURL)
			}
			return redis.Dial("tcp", redisURL)
		},
	}
}
