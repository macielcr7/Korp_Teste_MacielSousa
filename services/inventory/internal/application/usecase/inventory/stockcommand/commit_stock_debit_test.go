package stockcommand

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commandentity "github.com/macielcr7/korp-teste/services/inventory/internal/domain/entity/inventory/stockcommand"
	commandrepo "github.com/macielcr7/korp-teste/services/inventory/internal/domain/repository/inventory/stockcommand"
)

type committerStub struct {
	command commandentity.StockCommand
	result  commandrepo.CommitResult
	err     error
}

func (stub *committerStub) Commit(_ context.Context, command commandentity.StockCommand) (commandrepo.CommitResult, error) {
	stub.command = command
	return stub.result, stub.err
}

func TestCommitStockDebitCanonicalHashIsOrderIndependent(t *testing.T) {
	committed := committedCommand(t)
	firstRepo := &committerStub{result: commandrepo.CommitResult{Command: committed}}
	secondRepo := &committerStub{result: commandrepo.CommitResult{Command: committed}}

	first := NewCommitStockDebit(firstRepo)
	second := NewCommitStockDebit(secondRepo)
	_, err := first.Execute(context.Background(), CommitStockDebitInput{
		CommandID: "command", InvoiceID: "invoice",
		Items: []commandentity.Item{{ProductID: "b", Quantity: 1}, {ProductID: "a", Quantity: 2}},
	})
	require.NoError(t, err)
	_, err = second.Execute(context.Background(), CommitStockDebitInput{
		CommandID: "command", InvoiceID: "invoice",
		Items: []commandentity.Item{{ProductID: "a", Quantity: 2}, {ProductID: "b", Quantity: 1}},
	})
	require.NoError(t, err)

	assert.Equal(t, firstRepo.command.PayloadHash(), secondRepo.command.PayloadHash())
	assert.Len(t, firstRepo.command.PayloadHash(), 64)
}

func committedCommand(t *testing.T) commandentity.StockCommand {
	t.Helper()
	command, err := commandentity.Rehydrate(commandentity.Snapshot{
		CommandID: "command", InvoiceID: "invoice", PayloadHash: "hash", Status: commandentity.StatusCommitted,
		Items: []commandentity.Item{{ProductID: "product", Quantity: 1}}, CreatedAt: time.Now(),
	})
	require.NoError(t, err)
	return command
}

func TestCommitStockDebitRejectsDuplicateProduct(t *testing.T) {
	useCase := NewCommitStockDebit(&committerStub{})
	_, err := useCase.Execute(context.Background(), CommitStockDebitInput{
		CommandID: "command", InvoiceID: "invoice",
		Items: []commandentity.Item{{ProductID: "same", Quantity: 1}, {ProductID: "same", Quantity: 1}},
	})
	assert.ErrorIs(t, err, commandentity.ErrDuplicateProduct)
}
