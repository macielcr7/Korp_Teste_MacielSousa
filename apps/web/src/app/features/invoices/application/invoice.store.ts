import { inject, Injectable, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { catchError, defer, EMPTY, finalize, Subject, switchMap, tap } from 'rxjs';

import { ApiError } from '../../../core/errors/api-error';
import { Invoice, InvoiceListQuery } from '../domain/invoice';
import { BillingGateway } from './billing.gateway';

@Injectable({ providedIn: 'root' })
export class InvoiceStore {
  private readonly api = inject(BillingGateway);

  readonly invoices = signal<readonly Invoice[]>([]);
  readonly total = signal(0);
  readonly loading = signal(false);
  readonly loaded = signal(false);
  readonly error = signal<ApiError | null>(null);
  private readonly loadRequests = new Subject<InvoiceListQuery>();

  constructor() {
    this.loadRequests
      .pipe(
        switchMap((query) =>
          defer(() => {
            this.loading.set(true);
            this.error.set(null);
            return this.api.list(query);
          }).pipe(
            tap((collection) => {
              this.invoices.set(collection.items);
              this.total.set(collection.total);
              this.loaded.set(true);
            }),
            catchError((error: ApiError) => {
              this.error.set(error);
              this.invoices.set([]);
              this.total.set(0);
              return EMPTY;
            }),
            finalize(() => this.loading.set(false)),
          ),
        ),
        takeUntilDestroyed(),
      )
      .subscribe();
  }

  load(query?: InvoiceListQuery): void {
    this.loadRequests.next(query ?? { limit: 100, offset: 0 });
  }
}
