package cmd

import (
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
)

func newRedisOptions(cfg redisConfig) *redis.Options {
	addr := cfg.Addr
	if addr == "" {
		addr = defaultRedisAddr
	}
	return &redis.Options{
		Addr:     addr,
		DB:       cfg.DB,
		Password: cfg.Password,
		MaintNotificationsConfig: &maintnotifications.Config{
			Mode: maintnotifications.ModeDisabled,
		},
	}
}
