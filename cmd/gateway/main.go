package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/prehisle/yapi/internal/admin"
	"github.com/prehisle/yapi/internal/middleware"
	"github.com/prehisle/yapi/internal/proxy"
	"github.com/prehisle/yapi/pkg/config"
	"github.com/prehisle/yapi/pkg/rules"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	store, dbCloser := setupStore(ctx, cfg)
	defer func() {
		if dbCloser != nil {
			if err := dbCloser(); err != nil {
				log.Printf("database close error: %v", err)
			}
		}
	}()

	redisClient, cache, eventBus := setupRedis(ctx, cfg)
	if redisClient != nil {
		defer func() {
			if err := redisClient.Close(); err != nil {
				log.Printf("redis close error: %v", err)
			}
		}()
	}

	var serviceOpts []rules.ServiceOption
	if cache != nil {
		serviceOpts = append(serviceOpts, rules.WithCache(cache))
	}
	if eventBus != nil {
		serviceOpts = append(serviceOpts, rules.WithEventBus(eventBus))
	}

	ruleService := rules.NewService(store, serviceOpts...)
	ruleService.StartBackgroundSync(ctx)

	if err := seedDefaultRule(ctx, ruleService); err != nil {
		log.Printf("failed to seed default rule: %v", err)
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.RequestID(), middleware.AccessLogger(logger))

	adminService := admin.NewService(ruleService)
	adminHandler := admin.NewHandler(adminService)
	admin.RegisterRoutes(router.Group("/admin"), adminHandler)

	defaultTarget := mustParseURL(cfg.UpstreamBaseURL)
	proxyHandler := proxy.NewHandler(ruleService, proxy.WithDefaultTarget(defaultTarget), proxy.WithLogger(logger))
	proxy.RegisterRoutes(router.Group(""), proxyHandler)

	server := &http.Server{
		Addr:              ":" + cfg.GatewayPort,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("server shutdown error: %v", err)
		}
	}()

	log.Printf("gateway listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

func mustParseURL(raw string) *url.URL {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		log.Fatalf("invalid UPSTREAM_BASE_URL: %v", err)
	}
	return u
}

func seedDefaultRule(ctx context.Context, svc rules.Service) error {
	defaultRule := rules.Rule{
		ID:       "bootstrap-openai",
		Priority: 100,
		Matcher: rules.Matcher{
			PathPrefix: "/v1",
			Methods:    []string{"POST"},
		},
		Actions: rules.Actions{
			SetTargetURL: "https://api.openai.com",
		},
		Enabled: false,
	}
	_, err := svc.GetRule(ctx, defaultRule.ID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, rules.ErrRuleNotFound) {
		return err
	}
	return svc.UpsertRule(ctx, defaultRule)
}

func setupStore(ctx context.Context, cfg config.Config) (rules.Store, func() error) {
	if cfg.DatabaseDSN == "" {
		return rules.NewMemoryStore(), nil
	}
	gormLogger := logger.New(log.New(os.Stdout, "gorm: ", log.LstdFlags), logger.Config{
		SlowThreshold: time.Second,
		LogLevel:      logger.Warn,
	})
	db, err := gorm.Open(postgres.Open(cfg.DatabaseDSN), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("failed to get database connection: %v", err)
	}
	configureSQLDB(sqlDB)

	store := rules.NewDBStore(db)
	if err := store.AutoMigrate(ctx); err != nil {
		log.Fatalf("database migration failed: %v", err)
	}
	return store, sqlDB.Close
}

func configureSQLDB(db *sql.DB) {
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
}

func setupRedis(ctx context.Context, cfg config.Config) (*redis.Client, rules.Cache, rules.EventBus) {
	client := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		log.Printf("redis ping failed, fallback to in-memory cache: %v", err)
		_ = client.Close()
		return nil, nil, nil
	}
	cache := rules.NewRedisCache(client, "rules:all", 0)
	eventBus := rules.NewRedisEventBus(client, cfg.RedisChannel)
	return client, cache, eventBus
}
