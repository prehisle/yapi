package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config 汇总服务运行所需的环境变量配置。
type Config struct {
	GatewayPort         string
	UpstreamBaseURL     string
	DatabaseDSN         string
	RedisAddr           string
	RedisChannel        string
	RedisMaintMode      string
	AdminUsername       string
	AdminPassword       string
	AdminTokenSecret    string
	AdminTokenTTL       time.Duration
	AdminAllowedOrigins []string
}

const (
	defaultGatewayPort     = "8080"
	defaultRedisAddr       = "localhost:6379"
	defaultRedisChannel    = "rules:sync"
	RedisMaintModeDisabled = "disabled"
	RedisMaintModeAuto     = "auto"
	RedisMaintModeEnabled  = "enabled"
)

// Load 从环境变量解析配置。
func Load() Config {
	cfg := Config{
		GatewayPort:      lookupEnvOrDefault("GATEWAY_PORT", defaultGatewayPort),
		UpstreamBaseURL:  os.Getenv("UPSTREAM_BASE_URL"),
		DatabaseDSN:      os.Getenv("DATABASE_DSN"),
		RedisAddr:        lookupEnvOrDefault("REDIS_ADDR", defaultRedisAddr),
		RedisChannel:     lookupEnvOrDefault("REDIS_CHANNEL", defaultRedisChannel),
		RedisMaintMode:   lookupEnvOrDefault("REDIS_MAINT_NOTIFICATIONS_MODE", RedisMaintModeDisabled),
		AdminUsername:    os.Getenv("ADMIN_USERNAME"),
		AdminPassword:    os.Getenv("ADMIN_PASSWORD"),
		AdminTokenSecret: os.Getenv("ADMIN_TOKEN_SECRET"),
	}
	if rawAllowed := os.Getenv("ADMIN_ALLOWED_ORIGINS"); rawAllowed != "" {
		cfg.AdminAllowedOrigins = parseCSV(rawAllowed)
	}
	cfg.RedisMaintMode = normalizeMaintMode(cfg.RedisMaintMode)
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

func parseCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	var values []string
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func normalizeMaintMode(mode string) string {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	switch normalized {
	case "", RedisMaintModeDisabled:
		return RedisMaintModeDisabled
	case RedisMaintModeAuto, RedisMaintModeEnabled:
		return normalized
	default:
		log.Printf("warning: REDIS_MAINT_NOTIFICATIONS_MODE=%q 不受支持，将回退为 disabled", mode)
		return RedisMaintModeDisabled
	}
}
