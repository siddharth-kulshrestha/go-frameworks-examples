package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/siddharth-kulshrestha/taskforge/service"
	sqlcrepository "github.com/siddharth-kulshrestha/taskforge/sqlc/repository"
	"github.com/siddharth-kulshrestha/taskforge/transport/httpapi"
	organizationhttp "github.com/siddharth-kulshrestha/taskforge/transport/httpapi/organization"
)

func main() {
	if err := run(); err != nil {
		slog.Error("API server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("DATABASE_URL must be set")
	}

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelStartup()
	pool, err := pgxpool.New(startupCtx, databaseURL)
	if err != nil {
		return fmt.Errorf("create database pool: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(startupCtx); err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	orgRepository := sqlcrepository.NewOrganizationRepositoryImpl(pool)
	orgService := service.NewOrganizationService(orgRepository)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	router := httpapi.NewRouter(httpapi.Dependencies{
		Organizations: organizationhttp.NewHandler(orgService),
		Logger:        logger,
	})

	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	server := httpapi.NewServer(httpapi.ServerConfig{
		Addr:              addr,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}, router)
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", server.Addr, err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("starting HTTP server", "addr", server.Addr)
		serverErr <- server.Serve(listener)
	}()

	select {
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := server.Shutdown(shutdownCtx); err != nil {
		_ = server.Close()
		<-serverErr
		return fmt.Errorf("shut down HTTP server: %w", err)
	}
	if err := <-serverErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", err)
	}
	return nil
}
