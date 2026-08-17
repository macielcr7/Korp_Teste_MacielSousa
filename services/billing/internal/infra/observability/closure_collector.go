package observability

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var closureStatuses = [...]string{"PENDING", "PROCESSING", "RETRYING", "COMPLETED", "FAILED"}

// ClosureCollector reads durable closure metrics from PostgreSQL at scrape time.
// This makes the API scrape reflect work performed by the separate worker process.
type ClosureCollector struct {
	database         *sql.DB
	queryTimeout     time.Duration
	operations       *prometheus.Desc
	attempts         *prometheus.Desc
	oldestActionable *prometheus.Desc
}

// NewClosureCollector creates a database-backed closure collector.
func NewClosureCollector(database *sql.DB) *ClosureCollector {
	return &ClosureCollector{
		database:     database,
		queryTimeout: 2 * time.Second,
		operations: prometheus.NewDesc(
			"billing_closure_operations",
			"Current number of durable closure operations by status.",
			[]string{"status"}, nil,
		),
		attempts: prometheus.NewDesc(
			"billing_closure_attempts_total",
			"Total number of persisted closure processing attempts.",
			nil, nil,
		),
		oldestActionable: prometheus.NewDesc(
			"billing_closure_oldest_actionable_age_seconds",
			"Age in seconds of the oldest pending, retrying, or processing closure operation.",
			nil, nil,
		),
	}
}

// Describe implements prometheus.Collector.
func (collector *ClosureCollector) Describe(output chan<- *prometheus.Desc) {
	output <- collector.operations
	output <- collector.attempts
	output <- collector.oldestActionable
}

// Collect implements prometheus.Collector.
func (collector *ClosureCollector) Collect(output chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), collector.queryTimeout)
	defer cancel()

	counts, err := collector.operationCounts(ctx)
	if err != nil {
		output <- prometheus.NewInvalidMetric(collector.operations, err)
		return
	}
	for _, status := range closureStatuses {
		output <- prometheus.MustNewConstMetric(collector.operations, prometheus.GaugeValue, float64(counts[status]), status)
	}

	var attempts int64
	var oldestActionableAge float64
	err = collector.database.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(attempts), 0),
			COALESCE(EXTRACT(EPOCH FROM (
				CURRENT_TIMESTAMP - MIN(created_at) FILTER (
					WHERE status IN ('PENDING', 'PROCESSING', 'RETRYING')
				)
			)), 0)
		FROM closure_operations`).Scan(&attempts, &oldestActionableAge)
	if err != nil {
		output <- prometheus.NewInvalidMetric(collector.attempts, fmt.Errorf("collect closure totals: %w", err))
		return
	}
	output <- prometheus.MustNewConstMetric(collector.attempts, prometheus.CounterValue, float64(attempts))
	output <- prometheus.MustNewConstMetric(collector.oldestActionable, prometheus.GaugeValue, oldestActionableAge)
}

func (collector *ClosureCollector) operationCounts(ctx context.Context) (map[string]int64, error) {
	rows, err := collector.database.QueryContext(ctx, `
		SELECT status, COUNT(*)
		FROM closure_operations
		GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("collect closure status counts: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int64, len(closureStatuses))
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan closure status count: %w", err)
		}
		counts[status] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate closure status counts: %w", err)
	}
	return counts, nil
}
