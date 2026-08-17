package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"

	closureusecase "github.com/macielcr7/korp-teste/services/billing/internal/application/usecase/billing/closureoperation"
	inventoryhttp "github.com/macielcr7/korp-teste/services/billing/internal/infra/client/inventory/http"
	"github.com/macielcr7/korp-teste/services/billing/internal/infra/config"
	dbshared "github.com/macielcr7/korp-teste/services/billing/internal/infra/database/shared"
	operationpostgres "github.com/macielcr7/korp-teste/services/billing/internal/infra/repository/billing/closureoperation/postgres"
	invoicepostgres "github.com/macielcr7/korp-teste/services/billing/internal/infra/repository/billing/invoice/postgres"
)

const maximumWorkerRetryDelay = 30 * time.Second

func main() {
	_ = godotenv.Load()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	configuration, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(2)
	}
	database, err := dbshared.Open(dbshared.Config{Driver: "pgx", DSN: configuration.DatabaseURL, MaxOpenConns: 10, MaxIdleConns: 2, ConnMaxIdleTime: 5 * time.Minute, ConnMaxLifetime: 30 * time.Minute})
	if err != nil {
		slog.Error("database unavailable", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	operations := operationpostgres.New(database)
	invoices := invoicepostgres.New(database)
	inventory := inventoryhttp.New(configuration.InventoryBaseURL, configuration.InventoryInternalToken, &http.Client{Timeout: configuration.InventoryHTTPTimeout})
	process := closureusecase.NewProcess(operations, invoices, inventory, configuration.WorkerLeaseDuration, time.Now)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	slog.Info("billing worker started", "poll_interval", configuration.WorkerPollInterval, "lease_duration", configuration.WorkerLeaseDuration)
	runWorker(ctx, process, configuration.WorkerPollInterval, configuration.WorkerLeaseDuration)
}

type closureProcessor interface {
	Execute(context.Context) (closureusecase.ProcessOutput, error)
}

func runWorker(ctx context.Context, process closureProcessor, pollInterval, attemptTimeout time.Duration) {
	consecutiveFailures := 0
	for {
		output, err := executeAttempt(ctx, process, attemptTimeout)
		if err != nil {
			if ctx.Err() != nil {
				slog.Info("billing worker stopped")
				return
			}
			consecutiveFailures++
			retryDelay := workerRetryDelay(pollInterval, consecutiveFailures)
			slog.Error("closure processing failed", "operation_id", output.OperationID, "command_id", output.CommandID, "error", err, "cause", output.Cause, "retry_delay", retryDelay)
			if !waitForWorker(ctx, retryDelay) {
				slog.Info("billing worker stopped")
				return
			}
			continue
		}
		consecutiveFailures = 0
		if output.Cause != nil {
			slog.Warn("closure operation attempt did not complete", "operation_id", output.OperationID, "command_id", output.CommandID, "status", output.Status, "cause", output.Cause)
		}
		if output.Processed {
			slog.Info("closure operation processed", "operation_id", output.OperationID, "command_id", output.CommandID, "status", output.Status)
			continue
		}
		if !waitForWorker(ctx, pollInterval) {
			slog.Info("billing worker stopped")
			return
		}
	}
}

func executeAttempt(ctx context.Context, process closureProcessor, timeout time.Duration) (closureusecase.ProcessOutput, error) {
	attemptContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return process.Execute(attemptContext)
}

func workerRetryDelay(base time.Duration, consecutiveFailures int) time.Duration {
	if base <= 0 {
		base = time.Second
	}
	maximum := maximumWorkerRetryDelay
	if base > maximum {
		maximum = base
	}
	delay := base
	for attempt := 1; attempt < consecutiveFailures; attempt++ {
		if delay >= maximum/2 {
			return maximum
		}
		delay *= 2
	}
	if delay > maximum {
		return maximum
	}
	return delay
}

func waitForWorker(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
