package config

import (
	"log"
	"os"
	"strconv"
	"time"
)

// Config 汇总服务运行所需的环境变量配置。
type Config struct {
	GatewayPort     string
	UpstreamBaseURL string
	DatabaseDSN     string
	RedisAddr       string
	RedisChannel    string
	AdminUsername   string
	AdminPassword   string
	AdminTokenSecret string
	AdminTokenTTL    time.Duration
}

const (
	defaultGatewayPort  = "8080"
	defaultRedisAddr    = "localhost:6379"
	defaultRedisChannel = "rules:sync"
)

// Load 从环境变量解析配置。
func Load() Config {
	cfg := Config{
		GatewayPort:     lookupEnvOrDefault("GATEWAY_PORT", defaultGatewayPort),
		UpstreamBaseURL: os.Getenv("UPSTREAM_BASE_URL"),
		DatabaseDSN:     os.Getenv("DATABASE_DSN"),
		RedisAddr:       lookupEnvOrDefault("REDIS_ADDR", defaultRedisAddr),
		RedisChannel:    lookupEnvOrDefault("REDIS_CHANNEL", defaultRedisChannel),
		AdminUsername:   os.Getenv("ADMIN_USERNAME"),
		AdminPassword:   os.Getenv("ADMIN_PASSWORD"),
		AdminTokenSecret: os.Getenv("ADMIN_TOKEN_SECRET"),
	}
	if ttl := os.Getenv("ADMIN_TOKEN_TTL"); ttl != "" {
		if parsed, err := time.ParseDuration(ttl); err == nil {
			cfg.AdminTokenTTL = parsed
		} else if seconds, err := strconv.Atoi(ttl); err == nil {
			cfg.AdminTokenTTL = time.Duration(seconds) * time.Second
		} else {
			log.Printf("warning: ADMIN_TOKEN_TTL %q 无法解析，使用默认值", ttl)
		}
	}
	if cfg.AdminTokenTTL == 0 {
		cfg.AdminTokenTTL = 30 * time.Minute
	}
	if cfg.DatabaseDSN == "" {
		log.Println("warning: DATABASE_DSN 未设置，管理端规则持久化将不可用")
	}
	if cfg.AdminUsername == "" || cfg.AdminPassword == "" {
		log.Println("warning: 管理端未配置 ADMIN_USERNAME/ADMIN_PASSWORD，将默认允许匿名访问")
	}
	if cfg.AdminTokenSecret == "" {
		log.Println("warning: 管理端未配置 ADMIN_TOKEN_SECRET，将无法签发 JWT Access Token")
	}
	return cfg
}

func lookupEnvOrDefault(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}
