package repo

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// RedisConfig Redis配置
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// RedisClient Redis客户端接口
type RedisClient interface {
	Close() error
	Ping(ctx context.Context) *redis.StatusCmd
}

// NewRedis 创建Redis客户端
func NewRedis(cfg RedisConfig) (RedisClient, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return client, nil
}