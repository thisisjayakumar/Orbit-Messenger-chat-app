package data

import (
	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/presence-service/internal/conf"

	"github.com/redis/go-redis/v9"
)

// NewRedisClient creates a Redis client
func NewRedisClient(c *conf.Data) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:         c.Redis.Addr,
		ReadTimeout:  c.Redis.ReadTimeout.AsDuration(),
		WriteTimeout: c.Redis.WriteTimeout.AsDuration(),
	})
}
