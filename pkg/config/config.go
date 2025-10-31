package config

import (
	"log"
	"os"
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
	}
	if cfg.DatabaseDSN == "" {
		log.Println("warning: DATABASE_DSN 未设置，管理端规则持久化将不可用")
	}
	if cfg.AdminUsername == "" || cfg.AdminPassword == "" {
		log.Println("warning: 管理端未配置 ADMIN_USERNAME/ADMIN_PASSWORD，将默认允许匿名访问")
	}
	return cfg
}

func lookupEnvOrDefault(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}
