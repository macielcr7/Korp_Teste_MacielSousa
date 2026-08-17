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
	"github.com/prometheus/client_golang/prometheus"

	closureusecase "github.com/macielcr7/korp-teste/services/billing/internal/application/usecase/billing/closureoperation"
	invoiceusecase "github.com/macielcr7/korp-teste/services/billing/internal/application/usecase/billing/invoice"
	"github.com/macielcr7/korp-teste/services/billing/internal/infra/api"
	closurehandler "github.com/macielcr7/korp-teste/services/billing/internal/infra/api/handler/billing/closureoperation"
	invoicehandler "github.com/macielcr7/korp-teste/services/billing/internal/infra/api/handler/billing/invoice"
	inventoryhttp "github.com/macielcr7/korp-teste/services/billing/internal/infra/client/inventory/http"
	"github.com/macielcr7/korp-teste/services/billing/internal/infra/config"
	dbshared "github.com/macielcr7/korp-teste/services/billing/internal/infra/database/shared"
	idshared "github.com/macielcr7/korp-teste/services/billing/internal/infra/id/shared"
	"github.com/macielcr7/korp-teste/services/billing/internal/infra/observability"
	operationpostgres "github.com/macielcr7/korp-teste/services/billing/internal/infra/repository/billing/closureoperation/postgres"
	invoicepostgres "github.com/macielcr7/korp-teste/services/billing/internal/infra/repository/billing/invoice/postgres"
)

func main() {
	_ = godotenv.Load()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	configuration, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(2)
	}
	database, err := dbshared.Open(dbshared.Config{Driver: "pgx", DSN: configuration.DatabaseURL, MaxOpenConns: 20, MaxIdleConns: 5, ConnMaxIdleTime: 5 * time.Minute, ConnMaxLifetime: 30 * time.Minute})
	if err != nil {
		slog.Error("database unavailable", "error", err)
		os.Exit(1)
	}
	defer database.Close()
	metricsRegistry := prometheus.NewRegistry()
	metricsRegistry.MustRegister(prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}), observability.NewClosureCollector(database))
	httpMetrics, err := observability.NewHTTPMetrics(metricsRegistry)
	if err != nil {
		slog.Error("metrics setup failed", "error", err)
		os.Exit(1)
	}

	invoices := invoicepostgres.New(database)
	operations := operationpostgres.New(database)
	inventory := inventoryhttp.New(configuration.InventoryBaseURL, configuration.InventoryInternalToken, &http.Client{Timeout: configuration.InventoryHTTPTimeout})
	ids := idshared.UUIDGenerator{}
	createInvoice := invoiceusecase.NewCreate(invoices, inventory, ids, time.Now)
	getInvoice := invoiceusecase.NewGetDetail(invoices)
	listInvoices := invoiceusecase.NewList(invoices)
	getPrintable := invoiceusecase.NewGetPrintable(invoices)
	requestClosure := closureusecase.NewRequest(operations, ids, time.Now)
	getOperation := closureusecase.NewGet(operations)

	invoiceHTTP := invoicehandler.New(createInvoice, getInvoice, listInvoices, getPrintable, requestClosure)
	operationHTTP := closurehandler.New(getOperation)
	router := api.NewRouter(invoiceHTTP, operationHTTP, func(request *http.Request) error { return database.PingContext(request.Context()) }, httpMetrics, metricsRegistry)
	server := &http.Server{Addr: configuration.HTTPAddr, Handler: router, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}

	serverError := make(chan error, 1)
	go func() {
		slog.Info("billing API listening", "address", configuration.HTTPAddr)
		serverError <- server.ListenAndServe()
	}()
	shutdown, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	select {
	case <-shutdown.Done():
	case err := <-serverError:
		if !errors.Is(err, http.ErrServerClosed) {
			slog.Error("billing API stopped unexpectedly", "error", err)
			return
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("billing API graceful shutdown failed", "error", err)
	}
}
