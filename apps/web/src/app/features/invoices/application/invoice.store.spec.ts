import { TestBed } from '@angular/core/testing';
import { of, throwError } from 'rxjs';

import { ApiError } from '../../../core/errors/api-error';
import { BillingGateway } from './billing.gateway';
import { InvoiceStore } from './invoice.store';

describe('InvoiceStore', () => {
  it('clears results from the previous page when a remote request fails', () => {
    const invoice = {
      id: 'invoice-1',
      number: 1,
      status: 'OPEN' as const,
      createdAt: '2026-08-13T10:00:00Z',
      items: [],
    };
    const error = new ApiError('Serviço indisponível.', 503, undefined, undefined, true);
    const api = {
      list: vi
        .fn()
        .mockReturnValueOnce(of({ items: [invoice], total: 11, limit: 10, offset: 0 }))
        .mockReturnValueOnce(throwError(() => error)),
    };
    TestBed.configureTestingModule({
      providers: [InvoiceStore, { provide: BillingGateway, useValue: api }],
    });
    const store = TestBed.inject(InvoiceStore);

    store.load({ limit: 10, offset: 0 });
    expect(store.invoices()).toEqual([invoice]);

    store.load({ limit: 10, offset: 10 });

    expect(store.invoices()).toEqual([]);
    expect(store.total()).toBe(0);
    expect(store.error()).toBe(error);
  });
});
