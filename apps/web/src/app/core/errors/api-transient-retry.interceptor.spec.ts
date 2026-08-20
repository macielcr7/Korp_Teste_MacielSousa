import { HttpErrorResponse, HttpHeaders, HttpRequest, HttpResponse } from '@angular/common/http';
import { defer, firstValueFrom, of, throwError } from 'rxjs';

import { apiTransientRetryInterceptor } from './api-transient-retry.interceptor';

describe('apiTransientRetryInterceptor', () => {
  afterEach(() => vi.useRealTimers());

  it('retries a transient GET failure', async () => {
    vi.useFakeTimers();
    let attempts = 0;
    const request = new HttpRequest('GET', '/api/inventory/v1/products');
    const responsePromise = firstValueFrom(
      apiTransientRetryInterceptor(request, () =>
        defer(() => {
          attempts += 1;
          return attempts < 3
            ? throwError(() => new HttpErrorResponse({ status: 502 }))
            : of(new HttpResponse({ status: 200 }));
        }),
      ),
    );

    await vi.runAllTimersAsync();

    await expect(responsePromise).resolves.toMatchObject({ status: 200 });
    expect(attempts).toBe(3);
  });

  it('retries an idempotent POST failure', async () => {
    vi.useFakeTimers();
    let attempts = 0;
    const request = new HttpRequest('POST', '/api/billing/v1/invoices', {}, {
      headers: new HttpHeaders({ 'Idempotency-Key': 'invoice-key' }),
    });
    const responsePromise = firstValueFrom(
      apiTransientRetryInterceptor(request, () =>
        defer(() => {
          attempts += 1;
          return attempts === 1
            ? throwError(() => new HttpErrorResponse({ status: 0 }))
            : of(new HttpResponse({ status: 201 }));
        }),
      ),
    );

    await vi.runAllTimersAsync();

    await expect(responsePromise).resolves.toMatchObject({ status: 201 });
    expect(attempts).toBe(2);
  });

  it('does not retry a non-idempotent POST', async () => {
    let attempts = 0;
    const request = new HttpRequest('POST', '/api/inventory/v1/products', {});

    await expect(
      firstValueFrom(
        apiTransientRetryInterceptor(request, () => {
          attempts += 1;
          return throwError(() => new HttpErrorResponse({ status: 503 }));
        }),
      ),
    ).rejects.toMatchObject({ status: 503 });
    expect(attempts).toBe(1);
  });
});
