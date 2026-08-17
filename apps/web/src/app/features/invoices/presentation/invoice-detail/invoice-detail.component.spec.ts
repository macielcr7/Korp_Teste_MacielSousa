import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, provideRouter } from '@angular/router';
import { Observable, of, throwError } from 'rxjs';

import { IdempotencyStore } from '../../../../core/application/idempotency-store';
import { ApiError } from '../../../../core/errors/api-error';
import { SessionIdempotencyStore } from '../../../../core/infrastructure/session-idempotency-store';
import { BillingGateway } from '../../application/billing.gateway';
import { ClosureOperation, Invoice } from '../../domain/invoice';
import { InvoiceDetailComponent } from './invoice-detail.component';

describe('InvoiceDetailComponent', () => {
  const invoiceId = 'invoice-1';
  const storageKey = `invoice-close:${invoiceId}`;
  const invoice: Invoice = {
    id: invoiceId,
    number: 1,
    status: 'OPEN',
    createdAt: '2026-08-12T19:00:00Z',
    items: [
      {
        productId: 'product-1',
        productCode: 'SKU-001',
        productDescription: 'Produto de teste',
        quantity: 1,
      },
    ],
  };

  afterEach(() => {
    sessionStorage.removeItem(storageKey);
    vi.useRealTimers();
  });

  it('clears the idempotency key when the closure operation fails terminally', async () => {
    vi.useFakeTimers();
    const api = createApiMock(
      of({
        operationId: 'operation-1',
        status: 'FAILED' as const,
        lastError: 'Saldo insuficiente.',
        retryable: false,
      }),
    );
    const fixture = createFixture(api);

    fixture.detectChanges();
    sessionStorage.setItem(storageKey, 'original-key');
    closeButton(fixture.nativeElement).click();
    await vi.advanceTimersByTimeAsync(0);

    expect(api.close).toHaveBeenCalledWith(invoiceId, 'original-key');
    expect(sessionStorage.getItem(storageKey)).toBeNull();
    fixture.detectChanges();
    expect(fixture.nativeElement.textContent).toContain('Não foi possível emitir a nota');
    expect(fixture.nativeElement.textContent).toContain('Saldo insuficiente.');
    fixture.destroy();
  });

  it('preserves the idempotency key on a transient polling failure', async () => {
    vi.useFakeTimers();
    const api = createApiMock(
      throwError(() => new ApiError('Serviço indisponível.', 503, undefined, undefined, true)),
    );
    const fixture = createFixture(api);

    fixture.detectChanges();
    sessionStorage.setItem(storageKey, 'original-key');
    closeButton(fixture.nativeElement).click();
    await vi.advanceTimersByTimeAsync(0);

    expect(sessionStorage.getItem(storageKey)).toBe('original-key');
    fixture.destroy();
  });

  it('keeps a slow polling request active until it responds', async () => {
    vi.useFakeTimers();
    const slowProcessingOperation = new Observable<ClosureOperation>((subscriber) => {
      const timeout = setTimeout(() => {
        subscriber.next({ operationId: 'operation-1', status: 'PROCESSING' });
        subscriber.complete();
      }, 2000);
      return () => clearTimeout(timeout);
    });
    const api = createApiMock(slowProcessingOperation);
    const fixture = createFixture(api);

    fixture.detectChanges();
    sessionStorage.setItem(storageKey, 'original-key');
    closeButton(fixture.nativeElement).click();
    await vi.advanceTimersByTimeAsync(2000);

    expect(api.getClosureOperation).toHaveBeenCalledTimes(1);
    expect(sessionStorage.getItem(storageKey)).toBe('original-key');
    fixture.detectChanges();
    expect(fixture.nativeElement.textContent).toContain('Processando estoque');
    fixture.destroy();
  });

  it('retries a transient closure failure with the same idempotency key', async () => {
    vi.useFakeTimers();
    const transientError = new ApiError('Serviço indisponível.', 503, undefined, undefined, true);
    const api = createApiMock(of({ operationId: 'operation-1', status: 'PENDING' as const }));
    api.close
      .mockReturnValueOnce(throwError(() => transientError))
      .mockReturnValueOnce(of({ operationId: 'operation-1', status: 'PENDING' as const }));
    const fixture = createFixture(api);

    fixture.detectChanges();
    sessionStorage.setItem(storageKey, 'original-key');
    closeButton(fixture.nativeElement).click();
    fixture.detectChanges();

    const element = fixture.nativeElement as HTMLElement;
    const retryButton = [...element.querySelectorAll<HTMLButtonElement>('button')].find((button) =>
      button.textContent?.includes('Tentar novamente'),
    );
    retryButton?.click();
    await vi.advanceTimersByTimeAsync(0);

    expect(retryButton).toBeDefined();
    expect(api.close).toHaveBeenCalledTimes(2);
    expect(api.close).toHaveBeenNthCalledWith(1, invoiceId, 'original-key');
    expect(api.close).toHaveBeenNthCalledWith(2, invoiceId, 'original-key');
    expect(api.getById).toHaveBeenCalledTimes(1);
    fixture.destroy();
  });

  it('stops polling and clears the idempotency key on a terminal polling failure', async () => {
    vi.useFakeTimers();
    const api = createApiMock(
      throwError(
        () =>
          new ApiError(
            'A operação de emissão não foi encontrada.',
            404,
            'CLOSURE_OPERATION_NOT_FOUND',
            undefined,
            false,
          ),
      ),
    );
    const fixture = createFixture(api);

    fixture.detectChanges();
    sessionStorage.setItem(storageKey, 'original-key');
    closeButton(fixture.nativeElement).click();
    await vi.advanceTimersByTimeAsync(5000);
    fixture.detectChanges();

    expect(api.getClosureOperation).toHaveBeenCalledTimes(1);
    expect(sessionStorage.getItem(storageKey)).toBeNull();
    expect(closeButton(fixture.nativeElement).disabled).toBe(false);
    fixture.destroy();
  });

  it('clears a stale closure key when an open invoice has no active operation', () => {
    const api = createApiMock(of({ operationId: 'operation-1', status: 'PENDING' as const }));
    const fixture = createFixture(api);
    sessionStorage.setItem(storageKey, 'stale-key');

    fixture.detectChanges();

    expect(sessionStorage.getItem(storageKey)).toBeNull();
    fixture.destroy();
  });

  it('translates a legacy stock failure before showing it to the user', async () => {
    vi.useFakeTimers();
    const api = createApiMock(
      of({
        operationId: 'operation-1',
        status: 'FAILED' as const,
        lastError: 'insufficient product balance',
        retryable: false,
      }),
    );
    const fixture = createFixture(api);

    fixture.detectChanges();
    closeButton(fixture.nativeElement).click();
    await vi.advanceTimersByTimeAsync(0);
    fixture.detectChanges();

    expect(fixture.nativeElement.textContent).toContain(
      'Saldo insuficiente para um ou mais produtos da nota.',
    );
    expect(fixture.nativeElement.textContent).toContain('Não foi possível emitir a nota');
    expect(fixture.nativeElement.textContent).not.toContain('insufficient product balance');
    fixture.destroy();
  });

  function createApiMock(operationResult: Observable<ClosureOperation>) {
    return {
      getById: vi.fn(() => of(invoice)),
      close: vi.fn(() => of({ operationId: 'operation-1', status: 'PENDING' as const })),
      getClosureOperation: vi.fn(() => operationResult),
    };
  }

  function createFixture(api: ReturnType<typeof createApiMock>) {
    TestBed.configureTestingModule({
      imports: [InvoiceDetailComponent],
      providers: [
        provideRouter([]),
        { provide: BillingGateway, useValue: api },
        { provide: IdempotencyStore, useClass: SessionIdempotencyStore },
        {
          provide: ActivatedRoute,
          useValue: { snapshot: { paramMap: { get: () => invoiceId } } },
        },
      ],
    });

    return TestBed.createComponent(InvoiceDetailComponent);
  }

  function closeButton(element: HTMLElement): HTMLButtonElement {
    const button = element.querySelector<HTMLButtonElement>('button');
    if (!button) throw new Error('Close button was not rendered.');
    return button;
  }
});
