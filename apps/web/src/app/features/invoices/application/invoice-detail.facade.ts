import { computed, DestroyRef, inject, Injectable, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import {
  catchError,
  EMPTY,
  exhaustMap,
  finalize,
  of,
  Subject,
  takeUntil,
  tap,
  timer,
} from 'rxjs';

import { IdempotencyStore } from '../../../core/application/idempotency-store';
import { ApiError, localizeServerMessage } from '../../../core/errors/api-error';
import {
  ClosureOperation,
  ClosureOperationStatus,
  Invoice,
  isTerminalOperation,
} from '../domain/invoice';
import { BillingGateway } from './billing.gateway';

type InvoiceDetailErrorSource = 'load' | 'closure';

@Injectable()
export class InvoiceDetailFacade {
  private readonly billing = inject(BillingGateway);
  private readonly destroyRef = inject(DestroyRef);
  private readonly identities = inject(IdempotencyStore);
  private readonly closeRequests = new Subject<void>();
  private readonly stopPolling = new Subject<void>();
  private readonly printSubject = new Subject<string>();
  private invoiceId = '';

  readonly printRequested$ = this.printSubject.asObservable();
  readonly invoice = signal<Invoice | null>(null);
  readonly operation = signal<ClosureOperation | null>(null);
  readonly loading = signal(false);
  readonly processing = signal(false);
  readonly error = signal<ApiError | null>(null);
  private readonly errorSource = signal<InvoiceDetailErrorSource | null>(null);
  readonly errorTitle = computed(() =>
    this.errorSource() === 'closure'
      ? 'Não foi possível emitir a nota'
      : 'Não foi possível carregar a nota',
  );

  constructor() {
    this.closeRequests
      .pipe(
        tap(() => {
          this.processing.set(true);
          this.resetError();
        }),
        exhaustMap(() =>
          this.billing.close(this.invoiceId, this.getOrCreateIdentity()).pipe(
            catchError((error: ApiError) => {
              this.error.set(error);
              this.errorSource.set('closure');
              this.processing.set(false);
              if (!error.retryable) this.clearIdentity();
              return EMPTY;
            }),
          ),
        ),
        takeUntilDestroyed(),
      )
      .subscribe((operation) => {
        this.errorSource.set(null);
        this.operation.set(operation);
        this.startPolling(operation.operationId);
      });
  }

  initialize(invoiceId: string): void {
    this.invoiceId = invoiceId;
    this.loadInvoice();
  }

  loadInvoice(): void {
    this.loading.set(true);
    this.resetError();
    this.billing
      .getById(this.invoiceId)
      .pipe(
        catchError((error: ApiError) => {
          this.error.set(error);
          this.errorSource.set('load');
          return EMPTY;
        }),
        finalize(() => this.loading.set(false)),
      )
      .subscribe((invoice) => {
        this.invoice.set(invoice);
        const activeOperation = invoice.activeClosureOperation;
        if (activeOperation && !isTerminalOperation(activeOperation.status)) {
          this.operation.set(activeOperation);
          this.processing.set(true);
          this.startPolling(activeOperation.operationId);
          return;
        }
        this.operation.set(null);
        this.processing.set(false);
        this.clearIdentity();
      });
  }

  requestClose(): void {
    this.closeRequests.next();
  }

  retryLastAction(): void {
    if (this.errorSource() === 'closure') this.requestClose();
    else this.loadInvoice();
  }

  operationLabel(status: ClosureOperationStatus): string {
    const labels: Record<ClosureOperationStatus, string> = {
      PENDING: 'Aguardando processamento',
      PROCESSING: 'Processando estoque',
      RETRYING: 'Serviço de estoque indisponível. Tentaremos novamente.',
      COMPLETED: 'Nota emitida',
      FAILED: 'Falha no processamento',
    };
    return labels[status];
  }

  operationProgress(status: ClosureOperationStatus | undefined): number {
    const progress: Record<ClosureOperationStatus, number> = {
      PENDING: 25,
      PROCESSING: 55,
      RETRYING: 70,
      COMPLETED: 100,
      FAILED: 100,
    };
    return status ? progress[status] : 10;
  }

  stepState(step: ClosureOperationStatus): 'done' | 'active' | 'todo' | 'failed' {
    const current = this.operation()?.status ?? 'PENDING';
    if (current === 'FAILED') {
      if (step === 'FAILED') return 'failed';
      if (step === 'PENDING' || step === 'PROCESSING') return 'done';
      return 'todo';
    }
    const order: ClosureOperationStatus[] = ['PENDING', 'PROCESSING', 'RETRYING', 'COMPLETED'];
    const currentIndex = order.indexOf(current);
    const stepIndex = order.indexOf(step);
    if (stepIndex < currentIndex) return 'done';
    if (stepIndex === currentIndex) return current === 'COMPLETED' ? 'done' : 'active';
    return 'todo';
  }

  private startPolling(operationId: string): void {
    this.stopPolling.next();
    this.processing.set(true);
    timer(0, 1500)
      .pipe(
        exhaustMap(() =>
          this.billing.getClosureOperation(operationId).pipe(
            catchError((error: ApiError) => {
              this.error.set(error);
              this.errorSource.set('closure');
              if (!error.retryable) {
                this.clearIdentity();
                this.processing.set(false);
                this.stopPolling.next();
              }
              return of(null);
            }),
          ),
        ),
        tap((operation) => this.handlePolledOperation(operation)),
        takeUntil(this.stopPolling),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe();
  }

  private handlePolledOperation(operation: ClosureOperation | null): void {
    if (!operation) return;
    this.error.set(null);
    this.operation.set(operation);
    if (operation.status === 'COMPLETED') {
      this.clearIdentity();
      this.processing.set(false);
      this.stopPolling.next();
      this.printSubject.next(this.invoiceId);
    } else if (operation.status === 'FAILED') {
      this.clearIdentity();
      this.processing.set(false);
      this.errorSource.set('closure');
      this.error.set(
        new ApiError(
          localizeServerMessage(
            operation.lastError,
            undefined,
            'A emissão não foi concluída. Verifique os itens da nota antes de tentar novamente.',
          ),
          409,
          'CLOSURE_FAILED',
          undefined,
          operation.retryable,
          'Emissão rejeitada',
        ),
      );
      this.stopPolling.next();
    }
  }

  private getOrCreateIdentity(): string {
    return this.identities.getOrCreate(this.identityScope());
  }

  private clearIdentity(): void {
    this.identities.clear(this.identityScope());
  }

  private identityScope(): string {
    return `invoice-close:${this.invoiceId}`;
  }

  private resetError(): void {
    this.error.set(null);
    this.errorSource.set(null);
  }
}
