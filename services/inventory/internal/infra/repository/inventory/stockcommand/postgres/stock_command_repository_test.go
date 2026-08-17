package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commandentity "github.com/macielcr7/korp-teste/services/inventory/internal/domain/entity/inventory/stockcommand"
	commandrepo "github.com/macielcr7/korp-teste/services/inventory/internal/domain/repository/inventory/stockcommand"
)

func TestRepositoryCommitIsAtomicAndIdempotentAgainstPostgreSQL(t *testing.T) {
	fixture := newIntegrationFixture(t)
	firstProduct := fixture.createProduct(t, 10)
	secondProduct := fixture.createProduct(t, 5)
	command := fixture.createCommand(t, []commandentity.Item{
		{ProductID: firstProduct, Quantity: 3},
		{ProductID: secondProduct, Quantity: 2},
	})
	repository := New(fixture.database)

	result, err := repository.Commit(context.Background(), command)

	require.NoError(t, err)
	assert.False(t, result.Replayed)
	assert.Equal(t, commandentity.StatusCommitted, result.Command.Status())
	assert.Len(t, result.Command.Movements(), 2)
	assert.Equal(t, int64(7), fixture.balance(t, firstProduct))
	assert.Equal(t, int64(3), fixture.balance(t, secondProduct))

	replayed, err := repository.Commit(context.Background(), command)
	require.NoError(t, err)
	assert.True(t, replayed.Replayed)
	assert.Equal(t, int64(7), fixture.balance(t, firstProduct))
	assert.Equal(t, int64(3), fixture.balance(t, secondProduct))

	conflicting, err := commandentity.New(command.CommandID(), command.InvoiceID(), []commandentity.Item{
		{ProductID: firstProduct, Quantity: 4},
		{ProductID: secondProduct, Quantity: 2},
	})
	require.NoError(t, err)
	_, err = repository.Commit(context.Background(), conflicting)
	assert.ErrorIs(t, err, commandentity.ErrIdempotencyConflict)
}

func TestRepositoryCommitRejectsEntireCommandWhenOneBalanceIsInsufficient(t *testing.T) {
	fixture := newIntegrationFixture(t)
	firstProduct := fixture.createProduct(t, 10)
	secondProduct := fixture.createProduct(t, 1)
	command := fixture.createCommand(t, []commandentity.Item{
		{ProductID: firstProduct, Quantity: 3},
		{ProductID: secondProduct, Quantity: 2},
	})
	repository := New(fixture.database)

	result, err := repository.Commit(context.Background(), command)

	assert.ErrorIs(t, err, commandentity.ErrInsufficientStock)
	assert.Equal(t, commandentity.StatusRejected, result.Command.Status())
	assert.Empty(t, result.Command.Movements())
	assert.Equal(t, int64(10), fixture.balance(t, firstProduct))
	assert.Equal(t, int64(1), fixture.balance(t, secondProduct))

	persisted, err := repository.GetByID(context.Background(), command.CommandID())
	require.NoError(t, err)
	assert.Equal(t, commandentity.StatusRejected, persisted.Status())
	assert.Equal(t, commandentity.ErrorInsufficientStock, persisted.ErrorCode())
}

func TestRepositoryConcurrentIdempotentCommitDebitsOnlyOnce(t *testing.T) {
	fixture := newIntegrationFixture(t)
	productID := fixture.createProduct(t, 10)
	command := fixture.createCommand(t, []commandentity.Item{{ProductID: productID, Quantity: 4}})
	repository := New(fixture.database)

	results, errs := commitConcurrently(repository, command, command)

	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	assert.NotEqual(t, results[0].Replayed, results[1].Replayed)
	assert.Equal(t, int64(6), fixture.balance(t, productID))
}

func TestRepositoryConcurrentCommandsCannotOverdrawBalance(t *testing.T) {
	fixture := newIntegrationFixture(t)
	productID := fixture.createProduct(t, 10)
	first := fixture.createCommand(t, []commandentity.Item{{ProductID: productID, Quantity: 7}})
	second := fixture.createCommand(t, []commandentity.Item{{ProductID: productID, Quantity: 7}})
	repository := New(fixture.database)

	results, errs := commitConcurrently(repository, first, second)

	committed, rejected := 0, 0
	for index, err := range errs {
		switch {
		case err == nil:
			committed++
			assert.Equal(t, commandentity.StatusCommitted, results[index].Command.Status())
		case errors.Is(err, commandentity.ErrInsufficientStock):
			rejected++
			assert.Equal(t, commandentity.StatusRejected, results[index].Command.Status())
		default:
			require.NoError(t, err)
		}
	}
	assert.Equal(t, 1, committed)
	assert.Equal(t, 1, rejected)
	assert.Equal(t, int64(3), fixture.balance(t, productID))
}

func commitConcurrently(repository *Repository, commands ...commandentity.StockCommand) ([]commandrepo.CommitResult, []error) {
	results := make([]commandrepo.CommitResult, len(commands))
	errs := make([]error, len(commands))
	start := make(chan struct{})
	var waitGroup sync.WaitGroup
	for index := range commands {
		waitGroup.Add(1)
		go func(index int) {
			defer waitGroup.Done()
			<-start
			results[index], errs[index] = repository.Commit(context.Background(), commands[index])
		}(index)
	}
	close(start)
	waitGroup.Wait()
	return results, errs
}

type integrationFixture struct {
	database   *sql.DB
	productIDs []string
	commandIDs []string
}

func newIntegrationFixture(t *testing.T) *integrationFixture {
	t.Helper()
	databaseURL := os.Getenv("INVENTORY_INTEGRATION_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("INVENTORY_INTEGRATION_DATABASE_URL is not configured")
	}
	database, err := sql.Open("pgx", databaseURL)
	require.NoError(t, err)
	require.NoError(t, database.Ping())
	fixture := &integrationFixture{database: database}
	t.Cleanup(func() {
		fixture.cleanup(t)
		require.NoError(t, database.Close())
	})
	return fixture
}

func (fixture *integrationFixture) createProduct(t *testing.T, balance int64) string {
	t.Helper()
	productID := uuid.NewString()
	code := "IT-" + productID[:8]
	_, err := fixture.database.Exec(`
		INSERT INTO products (id, code, description, balance)
		VALUES ($1, $2, $3, $4)`, productID, code, "Stock command integration fixture", balance)
	require.NoError(t, err)
	fixture.productIDs = append(fixture.productIDs, productID)
	return productID
}

func (fixture *integrationFixture) createCommand(t *testing.T, items []commandentity.Item) commandentity.StockCommand {
	t.Helper()
	commandID, invoiceID := uuid.NewString(), uuid.NewString()
	command, err := commandentity.New(commandID, invoiceID, items)
	require.NoError(t, err)
	fixture.commandIDs = append(fixture.commandIDs, commandID)
	return command
}

func (fixture *integrationFixture) balance(t *testing.T, productID string) int64 {
	t.Helper()
	var balance int64
	require.NoError(t, fixture.database.QueryRow(`SELECT balance FROM products WHERE id = $1`, productID).Scan(&balance))
	return balance
}

func (fixture *integrationFixture) cleanup(t *testing.T) {
	t.Helper()
	for _, commandID := range fixture.commandIDs {
		for _, statement := range []string{
			`DELETE FROM stock_movements WHERE command_id = $1`,
			`DELETE FROM stock_command_items WHERE command_id = $1`,
			`DELETE FROM stock_commands WHERE command_id = $1`,
		} {
			_, err := fixture.database.Exec(statement, commandID)
			require.NoError(t, err, fmt.Sprintf("cleanup command %s", commandID))
		}
	}
	for _, productID := range fixture.productIDs {
		_, err := fixture.database.Exec(`DELETE FROM products WHERE id = $1`, productID)
		require.NoError(t, err, fmt.Sprintf("cleanup product %s", productID))
	}
}
