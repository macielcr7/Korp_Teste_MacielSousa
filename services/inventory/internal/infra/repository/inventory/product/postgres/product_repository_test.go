package postgres

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	productentity "github.com/macielcr7/korp-teste/services/inventory/internal/domain/entity/inventory/product"
	productrepo "github.com/macielcr7/korp-teste/services/inventory/internal/domain/repository/inventory/product"
)

func TestBuildListFilterUsesParametersAndEscapesSearchWildcards(t *testing.T) {
	minimum, maximum := int64(1), int64(5)

	where, arguments := buildListFilter(productrepo.ListCriteria{
		Search: "50%_off", MinimumBalance: &minimum, MaximumBalance: &maximum,
	})

	assert.Contains(t, where, "lower(code) LIKE")
	assert.Contains(t, where, "balance >= $2")
	assert.Contains(t, where, "balance <= $3")
	assert.Equal(t, []any{`50\%\_off`, int64(1), int64(5)}, arguments)
}

func TestBuildListFilterOmitsWhereWithoutFilters(t *testing.T) {
	where, arguments := buildListFilter(productrepo.ListCriteria{})

	assert.Empty(t, where)
	assert.Empty(t, arguments)
}

func TestRepositoryListAgainstPostgreSQL(t *testing.T) {
	database := openIntegrationDatabase(t)
	repository := New(database)
	targetID, decoyID := uuid.NewString(), uuid.NewString()
	suffix := strings.ReplaceAll(targetID, "-", "")[:12]
	t.Cleanup(func() {
		_, err := database.Exec(`DELETE FROM products WHERE id IN ($1, $2)`, targetID, decoyID)
		require.NoError(t, err)
	})

	target, err := productentity.New(targetID, "IT-"+suffix, "Integration 50%_off", 7, time.Now())
	require.NoError(t, err)
	decoy, err := productentity.New(decoyID, "IT-DECOY-"+suffix, "Integration 50XXoff", 9, time.Now())
	require.NoError(t, err)
	require.NoError(t, repository.Create(context.Background(), target))
	require.NoError(t, repository.Create(context.Background(), decoy))
	minimum := int64(6)

	result, err := repository.List(context.Background(), productrepo.ListCriteria{
		Search: "50%_off", MinimumBalance: &minimum, Limit: 10, Offset: 0,
	})

	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, int64(1), result.Total)
	assert.Equal(t, targetID, result.Items[0].ID())

	duplicate, err := productentity.New(uuid.NewString(), target.Code(), "Duplicate", 1, time.Now())
	require.NoError(t, err)
	assert.ErrorIs(t, repository.Create(context.Background(), duplicate), productentity.ErrDuplicateCode)

	found, err := repository.GetByID(context.Background(), targetID)
	require.NoError(t, err)
	assert.Equal(t, target.ID(), found.ID())
	assert.Equal(t, target.Code(), found.Code())
	assert.Equal(t, target.Description(), found.Description())
	assert.Equal(t, target.Balance(), found.Balance())
}

func openIntegrationDatabase(t *testing.T) *sql.DB {
	t.Helper()
	databaseURL := os.Getenv("INVENTORY_INTEGRATION_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("INVENTORY_INTEGRATION_DATABASE_URL is not configured")
	}
	database, err := sql.Open("pgx", databaseURL)
	require.NoError(t, err)
	require.NoError(t, database.Ping())
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	return database
}
