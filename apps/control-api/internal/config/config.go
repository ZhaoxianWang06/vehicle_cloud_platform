package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr      string
	MySQLDSN      string
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	RedisTLS      bool
	RequestLimit  time.Duration
}

func Load() (Config, error) {
	db, err := strconv.Atoi(getenv("REDIS_DB", "0"))
	if err != nil {
		return Config{}, fmt.Errorf("REDIS_DB: %w", err)
	}
	cfg := Config{
		HTTPAddr:      getenv("HTTP_ADDR", ":8080"),
		MySQLDSN:      os.Getenv("MYSQL_DSN"),
		RedisAddr:     getenv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword: os.Getenv("REDIS_PASSWORD"),
		RedisDB:       db,
		RedisTLS:      os.Getenv("REDIS_TLS") == "true",
		RequestLimit:  5 * time.Second,
	}
	if cfg.MySQLDSN == "" {
		return Config{}, fmt.Errorf("MYSQL_DSN 不能为空")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
