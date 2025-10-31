package main

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/prehisle/yapi/internal/admin"
	"github.com/prehisle/yapi/internal/proxy"
	"github.com/prehisle/yapi/pkg/rules"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	ruleStore := rules.NewMemoryStore()
	ruleService := rules.NewService(ruleStore)

	if err := seedDefaultRule(ctx, ruleService); err != nil {
		log.Printf("failed to seed default rule: %v", err)
	}

	adminHandler := admin.NewHandler(ruleService)
	admin.RegisterRoutes(router.Group("/admin"), adminHandler)

	defaultTarget := mustParseURL(os.Getenv("UPSTREAM_BASE_URL"))
	proxyHandler := proxy.NewHandler(ruleService, proxy.WithDefaultTarget(defaultTarget))
	proxy.RegisterRoutes(router.Group(""), proxyHandler)

	server := &http.Server{
		Addr:              ":" + serverPort(),
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

func serverPort() string {
	if port := os.Getenv("GATEWAY_PORT"); port != "" {
		return port
	}
	return "8080"
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
	return svc.UpsertRule(ctx, defaultRule)
}
