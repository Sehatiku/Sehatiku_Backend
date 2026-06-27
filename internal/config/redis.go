package config

import (
	"context"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func SetUpRedis(viper *viper.Viper, zap *zap.Logger) *redis.Client {
	ctx := context.Background()
	redisUrl := viper.GetString("REDIS_URL")
	opt, _ := redis.ParseURL(redisUrl)
	client := redis.NewClient(opt)

	client.Set(ctx, "foo", "bar", 0)
	val := client.Get(ctx, "foo").Val()
	print(val)
	_, err := client.Ping(ctx).Result()
	if err != nil {
		zap.Fatal("Failed to connect redis")
	}
	return client
}
