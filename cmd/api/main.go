// Command api is the entrypoint for the SaaS backend. It wires config,
// database, cache, and every domain module together, then runs the HTTP
// server and background job worker side by side until the process
// receives a shutdown signal.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"github.com/divyeshkakadiya/saas-backend/internal/auth"
	"github.com/divyeshkakadiya/saas-backend/internal/billing"
	"github.com/divyeshkakadiya/saas-backend/internal/config"
	"github.com/divyeshkakadiya/saas-backend/internal/db"
	"github.com/divyeshkakadiya/saas-backend/internal/httpserver"
	"github.com/divyeshkakadiya/saas-backend/internal/logger"
	"github.com/divyeshkakadiya/saas-backend/internal/tenant"
	"github.com/divyeshkakadiya/saas-backend/internal/worker"
)

func main() {
	// .env is optional — in production, real env vars are injected by the
	// platform (Docker, Fly.io, ECS) and no .env file will exist.
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config error", "error", err)
		os.Exit(1)
	}

	log := logger.New(cfg.Env)

	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	redisClient, err := db.ConnectRedis(cfg.RedisURL)
	if err != nil {
		log.Error("redis connection failed", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()

	// --- Wire domain modules ---
	tokens := auth.NewTokenManager(cfg.JWTSecret, cfg.AccessTokenTTL)

	authRepo := auth.NewRepository(database)
	authSvc := auth.NewService(authRepo, tokens, cfg.RefreshTTL)
	authHandler := auth.NewHandler(authSvc, log)

	tenantRepo := tenant.NewRepository(database)
	tenantSvc := tenant.NewService(tenantRepo)
	tenantHandler := tenant.NewHandler(tenantSvc, log)

	billingClient := billing.NewClient(cfg.StripeSecret)
	billingRepo := billing.NewRepository(database)
	billingHandler := billing.NewHandler(billingClient, billingRepo, cfg.StripeWebhook, log)

	// --- Background job worker ---
	queue := worker.NewQueue(redisClient, log)
	worker.RegisterEmailHandlers(queue, log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go queue.Run(ctx)

	router := httpserver.NewRouter(httpserver.Deps{
		Config:         cfg,
		DB:             database,
		Redis:          redisClient,
		Logger:         log,
		AuthHandler:    authHandler,
		Tokens:         tokens,
		TenantHandler:  tenantHandler,
		TenantRepo:     tenantRepo,
		BillingHandler: billingHandler,
	})

	if err := httpserver.Serve(ctx, cfg.Port, router, log); err != nil {
		log.Error("server exited with error", "error", err)
		os.Exit(1)
	}

	log.Info("shutdown complete")
}
