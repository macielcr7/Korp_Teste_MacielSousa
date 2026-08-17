package observability

import (
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

func TestClosureCollectorReadsDurableWorkerState(t *testing.T) {
	database, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT status, COUNT(*)
		FROM closure_operations
		GROUP BY status`)).WillReturnRows(
		sqlmock.NewRows([]string{"status", "count"}).
			AddRow("PENDING", int64(2)).
			AddRow("PROCESSING", int64(1)).
			AddRow("COMPLETED", int64(4)),
	)
	mock.ExpectQuery(`(?s)SELECT\s+COALESCE\(SUM\(attempts\), 0\).*FROM closure_operations`).
		WillReturnRows(sqlmock.NewRows([]string{"attempts", "age"}).AddRow(int64(9), float64(42.5)))
	collector := NewClosureCollector(database)

	err = testutil.CollectAndCompare(collector, strings.NewReader(`# HELP billing_closure_attempts_total Total number of persisted closure processing attempts.
# TYPE billing_closure_attempts_total counter
billing_closure_attempts_total 9
# HELP billing_closure_oldest_actionable_age_seconds Age in seconds of the oldest pending, retrying, or processing closure operation.
# TYPE billing_closure_oldest_actionable_age_seconds gauge
billing_closure_oldest_actionable_age_seconds 42.5
# HELP billing_closure_operations Current number of durable closure operations by status.
# TYPE billing_closure_operations gauge
billing_closure_operations{status="COMPLETED"} 4
billing_closure_operations{status="FAILED"} 0
billing_closure_operations{status="PENDING"} 2
billing_closure_operations{status="PROCESSING"} 1
billing_closure_operations{status="RETRYING"} 0
`), "billing_closure_operations", "billing_closure_attempts_total", "billing_closure_oldest_actionable_age_seconds")

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
