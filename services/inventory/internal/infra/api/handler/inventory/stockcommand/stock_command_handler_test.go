package stockcommand

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commandusecase "github.com/macielcr7/korp-teste/services/inventory/internal/application/usecase/inventory/stockcommand"
	commandentity "github.com/macielcr7/korp-teste/services/inventory/internal/domain/entity/inventory/stockcommand"
	commandrepo "github.com/macielcr7/korp-teste/services/inventory/internal/domain/repository/inventory/stockcommand"
)

type commitStub struct {
	input  commandusecase.CommitStockDebitInput
	result commandentity.StockCommand
	err    error
}

func (stub *commitStub) Execute(_ context.Context, input commandusecase.CommitStockDebitInput) (commandentity.StockCommand, error) {
	stub.input = input
	return stub.result, stub.err
}

type getStub struct{}

func (getStub) Execute(context.Context, string) (commandentity.StockCommand, error) {
	return commandentity.StockCommand{}, commandentity.ErrNotFound
}

type commitRepositoryStub struct {
	called bool
}

func (stub *commitRepositoryStub) Commit(_ context.Context, command commandentity.StockCommand) (commandrepo.CommitResult, error) {
	stub.called = true
	return commandrepo.CommitResult{Command: command}, nil
}

func TestCommit(t *testing.T) {
	command, err := commandentity.Rehydrate(commandentity.Snapshot{
		CommandID:   "53ee58d7-21d9-46be-9f7a-85d78990d161",
		InvoiceID:   "75f436f6-4cd4-4553-a88f-ef922fa5485a",
		PayloadHash: "hash", Status: commandentity.StatusCommitted,
		Items: []commandentity.Item{{ProductID: "7ee72534-5365-49e4-b978-494498508af3", Quantity: 2}}, CreatedAt: time.Now(),
	})
	require.NoError(t, err)
	committer := &commitStub{result: command}
	handler := New(committer, getStub{})
	body := `{"commandId":"53ee58d7-21d9-46be-9f7a-85d78990d161","invoiceId":"75f436f6-4cd4-4553-a88f-ef922fa5485a","items":[{"productId":"7ee72534-5365-49e4-b978-494498508af3","quantity":2}]}`
	request := httptest.NewRequest(http.MethodPost, "/internal/inventory/v1/stock-debits", strings.NewReader(body))
	response := httptest.NewRecorder()

	handler.Commit(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, int64(2), committer.input.Items[0].Quantity)
	assert.Contains(t, response.Body.String(), `"status":"COMMITTED"`)
}

func TestCommitMapsInsufficientStock(t *testing.T) {
	committer := &commitStub{err: commandentity.ErrInsufficientStock}
	handler := New(committer, getStub{})
	body := `{"commandId":"53ee58d7-21d9-46be-9f7a-85d78990d161","invoiceId":"75f436f6-4cd4-4553-a88f-ef922fa5485a","items":[{"productId":"7ee72534-5365-49e4-b978-494498508af3","quantity":2}]}`
	request := httptest.NewRequest(http.MethodPost, "/internal/inventory/v1/stock-debits", strings.NewReader(body))
	response := httptest.NewRecorder()

	handler.Commit(response, request)

	assert.Equal(t, http.StatusConflict, response.Code)
	assert.Contains(t, response.Body.String(), `"code":"INSUFFICIENT_STOCK"`)
	assert.Contains(t, response.Body.String(), `"detail":"Saldo insuficiente para um ou mais produtos da nota."`)
	assert.Contains(t, response.Body.String(), `"commandId":"53ee58d7-21d9-46be-9f7a-85d78990d161"`)
}

func TestCommitCanonicalizesUUIDsBeforeExecutingUseCase(t *testing.T) {
	command, err := commandentity.Rehydrate(commandentity.Snapshot{
		CommandID: "53ee58d7-21d9-46be-9f7a-85d78990d161",
		InvoiceID: "75f436f6-4cd4-4553-a88f-ef922fa5485a", PayloadHash: "hash",
		Status: commandentity.StatusCommitted,
		Items:  []commandentity.Item{{ProductID: "7ee72534-5365-49e4-b978-494498508af3", Quantity: 2}}, CreatedAt: time.Now(),
	})
	require.NoError(t, err)
	committer := &commitStub{result: command}
	handler := New(committer, getStub{})
	body := `{"commandId":"53EE58D7-21D9-46BE-9F7A-85D78990D161","invoiceId":"{75F436F6-4CD4-4553-A88F-EF922FA5485A}","items":[{"productId":"7EE72534536549E4B978494498508AF3","quantity":2}]}`
	request := httptest.NewRequest(http.MethodPost, "/internal/inventory/v1/stock-debits", strings.NewReader(body))
	response := httptest.NewRecorder()

	handler.Commit(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "53ee58d7-21d9-46be-9f7a-85d78990d161", committer.input.CommandID)
	assert.Equal(t, "75f436f6-4cd4-4553-a88f-ef922fa5485a", committer.input.InvoiceID)
	assert.Equal(t, "7ee72534-5365-49e4-b978-494498508af3", committer.input.Items[0].ProductID)
}

func TestCommitRejectsEquivalentDuplicateProductUUIDs(t *testing.T) {
	repository := &commitRepositoryStub{}
	handler := New(commandusecase.NewCommitStockDebit(repository), getStub{})
	body := `{"commandId":"53ee58d7-21d9-46be-9f7a-85d78990d161","invoiceId":"75f436f6-4cd4-4553-a88f-ef922fa5485a","items":[{"productId":"7ee72534-5365-49e4-b978-494498508af3","quantity":1},{"productId":"7EE72534-5365-49E4-B978-494498508AF3","quantity":1}]}`
	request := httptest.NewRequest(http.MethodPost, "/internal/inventory/v1/stock-debits", strings.NewReader(body))
	response := httptest.NewRecorder()

	handler.Commit(response, request)

	assert.Equal(t, http.StatusUnprocessableEntity, response.Code)
	assert.Contains(t, response.Body.String(), `"code":"INVALID_STOCK_COMMAND"`)
	assert.False(t, repository.called)
}
