package store

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewRedis connects to Valkey/Redis (used for the RBAC permission cache).
func NewRedis(addr, password string) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return client, nil
}
