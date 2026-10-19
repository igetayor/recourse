package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/igetayor/recourse/internal/api"
	"github.com/igetayor/recourse/internal/auth"
	"github.com/igetayor/recourse/internal/config"
	"github.com/igetayor/recourse/internal/db"
	"github.com/igetayor/recourse/internal/graph"
	"github.com/igetayor/recourse/internal/logger"
)

func main() {
	cfg := config.LoadFromEnv()
	log := logger.InitLogger(cfg.LogLevel, cfg.ServiceName)

	log.Info("starting recourse service",
		slog.String("port", cfg.Port),
		slog.String("service", cfg.ServiceName),
	)

	// ─── Database ────────────────────────────────────────────────────
	ctx := context.Background()
	pool, err := db.New(ctx, cfg, log)
	if err != nil {
		log.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// ─── Repositories ────────────────────────────────────────────────
	userRepo := db.NewUserRepository(pool)
	disputeRepo := db.NewDisputeRepository(pool)

	// ─── Auth ────────────────────────────────────────────────────────
	jwtService := auth.NewJWTService(cfg)

	// ─── GraphQL Schema ──────────────────────────────────────────────
	resolver := &graph.Resolver{
		UserRepo:    userRepo,
		DisputeRepo: disputeRepo,
		JWTService:  jwtService,
		Cfg:         cfg,
	}
	schema := graph.NewSchema(resolver)

	// ─── HTTP Server ─────────────────────────────────────────────────
	router := api.NewRouter(cfg, schema, jwtService)
	srv := &http.Server{
		Addr:         cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// ─── Graceful Shutdown ───────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		log.Info("http server listening", slog.String("addr", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-quit
	log.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}

	log.Info("server stopped gracefully")
	fmt.Println("Goodbye.")
}
