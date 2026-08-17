import { inject, Injectable, signal } from '@angular/core';
import { catchError, EMPTY, finalize } from 'rxjs';

import { ApiError } from '../../../core/errors/api-error';
import { Invoice } from '../domain/invoice';
import { BillingGateway } from './billing.gateway';

@Injectable()
export class InvoicePrintFacade {
  private readonly billing = inject(BillingGateway);

  readonly invoice = signal<Invoice | null>(null);
  readonly loading = signal(false);
  readonly error = signal<ApiError | null>(null);

  load(invoiceId: string): void {
    this.loading.set(true);
    this.error.set(null);
    this.billing
      .getPrintable(invoiceId)
      .pipe(
        catchError((error: ApiError) => {
          this.error.set(error);
          return EMPTY;
        }),
        finalize(() => this.loading.set(false)),
      )
      .subscribe((invoice) => this.invoice.set(invoice));
  }
}
