package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPPort string

	MySQLDSN string

	RedisAddr     string
	RedisPassword string
	RedisDB       int
	RedisTasksTTL time.Duration

	JWTSecret string

	RateLimitPerMin int

	InviteTTL time.Duration

	InviteNotifyURL string
}

func Load() (Config, error) {
	cfg := Config{
		HTTPPort: getStr("HTTP_PORT", "8080"),

		MySQLDSN: getStr("MYSQL_DSN", ""),

		RedisAddr:     getStr("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getStr("REDIS_PASSWORD", ""),
		RedisDB:       getInt("REDIS_DB", 0),
		RedisTasksTTL: time.Duration(getInt("REDIS_TASKS_TTL_MIN", 5)) * time.Minute,

		JWTSecret: getStr("JWT_SECRET", ""),

		RateLimitPerMin: getInt("RATE_LIMIT_PER_MIN", 100),

		InviteTTL: time.Duration(getInt("INVITE_TTL_MIN", 1440)) * time.Minute,

		InviteNotifyURL: getStr("INVITE_NOTIFY_URL", ""),
	}

	if cfg.MySQLDSN == "" {
		return Config{}, fmt.Errorf("MYSQL_DSN is required")
	}
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}
	return cfg, nil
}

func getStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return i
}
