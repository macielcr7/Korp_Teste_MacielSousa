package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	promclient "github.com/prometheus/client_golang/prometheus"

	"github.com/macielcr7/korp-teste/services/inventory/internal/application/service/shared"
	productusecase "github.com/macielcr7/korp-teste/services/inventory/internal/application/usecase/inventory/product"
	commandusecase "github.com/macielcr7/korp-teste/services/inventory/internal/application/usecase/inventory/stockcommand"
	"github.com/macielcr7/korp-teste/services/inventory/internal/infra/api"
	producthandler "github.com/macielcr7/korp-teste/services/inventory/internal/infra/api/handler/inventory/product"
	commandhandler "github.com/macielcr7/korp-teste/services/inventory/internal/infra/api/handler/inventory/stockcommand"
	"github.com/macielcr7/korp-teste/services/inventory/internal/infra/config"
	dbshared "github.com/macielcr7/korp-teste/services/inventory/internal/infra/database/shared"
	idshared "github.com/macielcr7/korp-teste/services/inventory/internal/infra/id/shared"
	metricsprometheus "github.com/macielcr7/korp-teste/services/inventory/internal/infra/observability/prometheus"
	productpostgres "github.com/macielcr7/korp-teste/services/inventory/internal/infra/repository/inventory/product/postgres"
	commandpostgres "github.com/macielcr7/korp-teste/services/inventory/internal/infra/repository/inventory/stockcommand/postgres"
)

func main() {
	_ = godotenv.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(2)
	}

	database, err := dbshared.Open(dbshared.Config{
		Driver: "pgx", DSN: cfg.DatabaseURL,
		MaxOpenConns: cfg.DatabaseMaxOpen, MaxIdleConns: cfg.DatabaseMaxIdle,
		ConnMaxIdleTime: cfg.DatabaseMaxIdleTime, ConnMaxLifetime: cfg.DatabaseMaxLifetime,
	})
	if err != nil {
		logger.Error("database initialization failed", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	productRepository := productpostgres.New(database)
	commandRepository := commandpostgres.New(database)
	metrics := metricsprometheus.New(promclient.DefaultRegisterer, promclient.DefaultGatherer)
	instrumentedCommandCommitter := metricsprometheus.NewStockCommandCommitter(commandRepository, metrics)

	createProduct := productusecase.NewCreateProduct(productRepository, idshared.UUIDGenerator{}, shared.SystemClock{})
	getProduct := productusecase.NewGetProduct(productRepository)
	listProducts := productusecase.NewListProducts(productRepository)
	commitStockDebit := commandusecase.NewCommitStockDebit(instrumentedCommandCommitter)
	getStockCommand := commandusecase.NewGetStockCommandResult(commandRepository)

	productHTTPHandler := producthandler.New(createProduct, getProduct, listProducts)
	commandHTTPHandler := commandhandler.New(commitStockDebit, getStockCommand)
	publicRouter := api.NewPublicRouter(logger, database, productHTTPHandler, metrics)
	internalRouter := api.NewInternalRouter(logger, productHTTPHandler, commandHTTPHandler, metrics, cfg.InternalAPIToken)

	publicServer := &http.Server{
		Addr: cfg.HTTPAddr, Handler: publicRouter,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	internalServer := &http.Server{
		Addr: cfg.InternalHTTPAddr, Handler: internalRouter,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverError := make(chan error, 2)
	go func() {
		logger.Info("inventory public HTTP server listening", "address", cfg.HTTPAddr)
		serverError <- publicServer.ListenAndServe()
	}()
	go func() {
		logger.Info("inventory internal HTTP server listening", "address", cfg.InternalHTTPAddr)
		serverError <- internalServer.ListenAndServe()
	}()

	signalContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	select {
	case <-signalContext.Done():
		logger.Info("inventory service shutdown requested")
	case err := <-serverError:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("inventory HTTP server failed", "error", err)
			return
		}
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for name, server := range map[string]*http.Server{"public": publicServer, "internal": internalServer} {
		if err := server.Shutdown(shutdownContext); err != nil {
			logger.Error("inventory HTTP server shutdown failed", "server", name, "error", err)
		}
	}
}
