package main

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	llmux "github.com/blueberrycongee/llmux"
	"github.com/blueberrycongee/llmux/caches/dual"
	"github.com/blueberrycongee/llmux/caches/memory"
	"github.com/blueberrycongee/llmux/caches/redis"
	"github.com/blueberrycongee/llmux/internal/config"
)

func buildCacheOptions(cfg *config.CacheConfig, logger *slog.Logger) ([]llmux.Option, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, nil
	}

	cacheType := strings.ToLower(cfg.Type)
	if cacheType == "" {
		cacheType = "local"
	}

	var cacheInstance llmux.Cache
	switch cacheType {
	case "local", "memory":
		cacheInstance = memory.New(memory.Config{
			MaxSize:         cfg.Memory.MaxSize,
			DefaultTTL:      cfg.Memory.DefaultTTL,
			MaxItemSize:     cfg.Memory.MaxItemSize,
			CleanupInterval: cfg.Memory.CleanupInterval,
		})
	case "redis":
		redisCache, err := redis.New(buildRedisCacheConfig(*cfg))
		if err != nil {
			return nil, err
		}
		cacheInstance = redisCache
	case "dual":
		local := memory.New(memory.Config{
			MaxSize:         cfg.Memory.MaxSize,
			DefaultTTL:      cfg.Memory.DefaultTTL,
			MaxItemSize:     cfg.Memory.MaxItemSize,
			CleanupInterval: cfg.Memory.CleanupInterval,
		})
		remote, err := redis.New(buildRedisCacheConfig(*cfg))
		if err != nil {
			return nil, err
		}
		dualCfg := dual.DefaultConfig()
		if cfg.Memory.DefaultTTL > 0 {
			dualCfg.LocalTTL = cfg.Memory.DefaultTTL
		}
		if cfg.TTL > 0 {
			dualCfg.RedisTTL = cfg.TTL
		}
		cacheInstance = dual.New(local, remote, dualCfg)
	default:
		return nil, fmt.Errorf("unsupported cache type: %s", cfg.Type)
	}

	opts := []llmux.Option{
		llmux.WithCache(cacheInstance),
		llmux.WithCacheTypeLabel(cacheType),
	}
	if cfg.TTL > 0 {
		opts = append(opts, llmux.WithCacheTTL(cfg.TTL))
	}

	logger.Info("cache enabled", "type", cacheType)
	return opts, nil
}

func buildRedisCacheConfig(cfg config.CacheConfig) redis.Config {
	redisCfg := redis.Config{
		Addr:           cfg.Redis.Addr,
		Password:       cfg.Redis.Password,
		DB:             cfg.Redis.DB,
		ClusterAddrs:   cfg.Redis.ClusterAddrs,
		SentinelAddrs:  cfg.Redis.SentinelAddrs,
		SentinelMaster: cfg.Redis.SentinelMaster,
		Namespace:      cfg.Namespace,
		DialTimeout:    cfg.Redis.DialTimeout,
		ReadTimeout:    cfg.Redis.ReadTimeout,
		WriteTimeout:   cfg.Redis.WriteTimeout,
		PoolSize:       cfg.Redis.PoolSize,
		MinIdleConns:   cfg.Redis.MinIdleConns,
		MaxRetries:     cfg.Redis.MaxRetries,
	}
	if cfg.TTL > 0 {
		redisCfg.DefaultTTL = cfg.TTL
	}
	if redisCfg.DefaultTTL == 0 {
		redisCfg.DefaultTTL = time.Hour
	}
	return redisCfg
}
