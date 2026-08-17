package closureoperation

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"github.com/macielcr7/korp-teste/services/billing/internal/application/service/billing/inventorygateway"
	operationentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/closureoperation"
	operationrepository "github.com/macielcr7/korp-teste/services/billing/internal/domain/repository/billing/closureoperation"
	invoicerepository "github.com/macielcr7/korp-teste/services/billing/internal/domain/repository/billing/invoice"
)

// ProcessOutput reports whether a job was found and its resulting state.
type ProcessOutput struct {
	Processed   bool
	OperationID string
	CommandID   string
	Status      operationentity.Status
	Cause       error
}

// Process leases and processes one durable closure operation.
type Process struct {
	operations    operationrepository.Processor
	invoices      invoicerepository.Finder
	inventory     inventorygateway.DebitCommitter
	leaseDuration time.Duration
	now           func() time.Time
}

func NewProcess(operations operationrepository.Processor, invoices invoicerepository.Finder, inventory inventorygateway.DebitCommitter, leaseDuration time.Duration, now func() time.Time) *Process {
	return &Process{operations: operations, invoices: invoices, inventory: inventory, leaseDuration: leaseDuration, now: now}
}

func (useCase *Process) Execute(ctx context.Context) (ProcessOutput, error) {
	now := useCase.now()
	operation, err := useCase.operations.AcquireNext(ctx, now, useCase.leaseDuration)
	if errors.Is(err, operationrepository.ErrNoOperationAvailable) {
		return ProcessOutput{}, nil
	}
	if err != nil {
		return ProcessOutput{}, fmt.Errorf("acquire closure operation: %w", err)
	}
	output := ProcessOutput{
		Processed:   true,
		OperationID: operation.ID(),
		CommandID:   operation.CommandID(),
		Status:      operation.Status(),
	}

	inv, err := useCase.invoices.FindByID(ctx, operation.InvoiceID())
	if err != nil {
		if errors.Is(err, invoicerepository.ErrNotFound) {
			output.Status = operationentity.StatusFailed
			output.Cause = err
			return output, useCase.fail(ctx, operation, "A nota não foi encontrada.")
		}
		output.Status = operationentity.StatusRetrying
		output.Cause = fmt.Errorf("load invoice: %w", err)
		return output, useCase.retry(ctx, operation, retryMessage)
	}
	items := make([]inventorygateway.DebitItem, 0, len(inv.Items()))
	for _, item := range inv.Items() {
		items = append(items, inventorygateway.DebitItem{ProductID: item.ProductID(), Quantity: item.Quantity()})
	}
	err = useCase.inventory.CommitDebit(ctx, inventorygateway.DebitCommand{CommandID: operation.CommandID(), InvoiceID: inv.ID(), Items: items})
	if err != nil {
		var rejected *inventorygateway.RejectedError
		if errors.As(err, &rejected) {
			output.Status = operationentity.StatusFailed
			output.Cause = err
			return output, useCase.fail(ctx, operation, rejectedMessage(rejected.Code))
		}
		output.Status = operationentity.StatusRetrying
		output.Cause = err
		return output, useCase.retry(ctx, operation, retryMessage)
	}

	completedAt := useCase.now()
	if err := inv.Close(completedAt); err != nil {
		return output, err
	}
	if err := operation.MarkCompleted(completedAt); err != nil {
		return output, err
	}
	if err := useCase.operations.CompleteWithInvoice(ctx, operation, inv); err != nil {
		return output, fmt.Errorf("complete closure operation: %w", err)
	}
	output.Status = operationentity.StatusCompleted
	return output, nil
}

const retryMessage = "O serviço de estoque não respondeu. A emissão será tentada novamente automaticamente."

func (useCase *Process) retry(ctx context.Context, operation operationentity.Operation, message string) error {
	now := useCase.now()
	if err := operation.MarkRetrying(message, now.Add(backoff(operation.Attempts(), operation.ID())), now); err != nil {
		return err
	}
	if err := useCase.operations.MarkRetrying(ctx, operation); err != nil {
		return fmt.Errorf("persist closure retry: %w", err)
	}
	return nil
}

func rejectedMessage(code string) string {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "INSUFFICIENT_STOCK", "INSUFFICIENT_BALANCE":
		return "Saldo insuficiente para um ou mais produtos da nota."
	case "PRODUCT_NOT_FOUND":
		return "Um dos produtos da nota não foi encontrado."
	case "INVALID_STOCK_COMMAND":
		return "Os dados da baixa de estoque são inválidos. Revise os produtos e as quantidades da nota."
	case "IDEMPOTENCY_CONFLICT":
		return "A baixa de estoque desta nota entrou em conflito com uma solicitação anterior."
	default:
		return "O estoque rejeitou a baixa. Verifique os produtos e as quantidades da nota antes de tentar novamente."
	}
}

func (useCase *Process) fail(ctx context.Context, operation operationentity.Operation, message string) error {
	now := useCase.now()
	if err := operation.MarkFailed(message, now); err != nil {
		return err
	}
	if err := useCase.operations.MarkFailed(ctx, operation); err != nil {
		return fmt.Errorf("persist closure failure: %w", err)
	}
	return nil
}

func backoff(attempt int, seed string) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	exponent := attempt - 1
	if exponent > 6 {
		exponent = 6
	}
	base := time.Second * time.Duration(1<<exponent)
	if base > time.Minute {
		base = time.Minute
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(seed))
	jitter := time.Duration(hash.Sum32()%250) * base / 1000
	return base + jitter
}
