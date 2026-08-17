import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { BillingApiService } from './billing-api.service';

describe('BillingApiService', () => {
  let service: BillingApiService;
  let http: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    service = TestBed.inject(BillingApiService);
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => http.verify());

  it('sends remote invoice filters and maps the paginated collection', () => {
    let total: number | undefined;

    service
      .list({ status: 'CLOSED', limit: 10, offset: 10 })
      .subscribe((collection) => (total = collection.total));

    const request = http.expectOne('/api/billing/v1/invoices?status=CLOSED&limit=10&offset=10');
    expect(request.request.method).toBe('GET');
    request.flush({ items: [], total: 23, limit: 10, offset: 10 });
    expect(total).toBe(23);
  });

  it('sends an idempotency key when closing an invoice', () => {
    service.close('invoice-1', 'request-123').subscribe();

    const request = http.expectOne('/api/billing/v1/invoices/invoice-1/close');
    expect(request.request.method).toBe('POST');
    expect(request.request.headers.get('Idempotency-Key')).toBe('request-123');
    request.flush({ operationId: 'operation-1', status: 'PENDING' });
  });

  it('sends an idempotency key when creating an invoice', () => {
    service.create({ items: [{ productId: 'product-1', quantity: 2 }] }, 'create-123').subscribe();

    const request = http.expectOne('/api/billing/v1/invoices');
    expect(request.request.method).toBe('POST');
    expect(request.request.headers.get('Idempotency-Key')).toBe('create-123');
    expect(request.request.body).toEqual({
      items: [{ productId: 'product-1', quantity: 2 }],
    });
    request.flush({
      id: 'invoice-1',
      number: 1,
      status: 'OPEN',
      createdAt: '2026-08-13T10:00:00Z',
      items: [],
    });
  });

  it('uses the printable contract when preparing a document', () => {
    service.getPrintable('invoice-1').subscribe();

    const request = http.expectOne('/api/billing/v1/invoices/invoice-1/printable');
    expect(request.request.method).toBe('GET');
    request.flush({
      id: 'invoice-1',
      number: 1,
      status: 'CLOSED',
      createdAt: '2026-08-13T10:00:00Z',
      closedAt: '2026-08-13T10:01:00Z',
      items: [],
    });
  });

  it('maps the closure operation resource id to the domain operationId', () => {
    let operationId: string | undefined;
    service.getClosureOperation('operation-1').subscribe((operation) => {
      operationId = operation.operationId;
    });

    const request = http.expectOne('/api/billing/v1/closure-operations/operation-1');
    expect(request.request.method).toBe('GET');
    request.flush({
      id: 'operation-1',
      invoiceId: 'invoice-1',
      status: 'PROCESSING',
      attempts: 1,
      nextAttemptAt: '2026-08-13T10:00:01Z',
      retryable: false,
      createdAt: '2026-08-13T10:00:00Z',
      updatedAt: '2026-08-13T10:00:01Z',
    });

    expect(operationId).toBe('operation-1');
  });

  it('maps API item code and description to the invoice domain model', () => {
    let result:
      | {
          readonly items: readonly {
            readonly productCode: string;
            readonly productDescription: string;
            readonly quantity: number;
          }[];
        }
      | undefined;

    service.getById('invoice-1').subscribe((invoice) => (result = invoice));

    http.expectOne('/api/billing/v1/invoices/invoice-1').flush({
      id: 'invoice-1',
      number: 42,
      status: 'CLOSED',
      createdAt: '2026-08-12T19:00:00Z',
      closedAt: '2026-08-12T19:01:00Z',
      items: [
        {
          productId: 'product-1',
          code: 'SKU-001',
          description: 'Produto de teste',
          quantity: 3,
        },
      ],
    });

    expect(result?.items[0]).toEqual({
      productId: 'product-1',
      productCode: 'SKU-001',
      productDescription: 'Produto de teste',
      quantity: 3,
    });
  });
});
