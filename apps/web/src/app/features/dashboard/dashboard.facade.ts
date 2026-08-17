import { DestroyRef, inject, Injectable, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { catchError, EMPTY, finalize, forkJoin } from 'rxjs';

import { ApiError } from '../../core/errors/api-error';
import { BillingGateway } from '../invoices/application/billing.gateway';
import { Invoice } from '../invoices/domain/invoice';
import { ProductGateway } from '../products/application/product.gateway';
import { Product } from '../products/domain/product';

@Injectable()
export class DashboardFacade {
  private readonly billing = inject(BillingGateway);
  private readonly destroyRef = inject(DestroyRef);
  private readonly products = inject(ProductGateway);

  readonly today = new Date();
  readonly totalProducts = signal(0);
  readonly attentionStockTotal = signal(0);
  readonly openInvoicesTotal = signal(0);
  readonly closedInvoicesTotal = signal(0);
  readonly recentInvoices = signal<readonly Invoice[]>([]);
  readonly criticalProducts = signal<readonly Product[]>([]);
  readonly loading = signal(false);
  readonly error = signal<ApiError | null>(null);

  reload(): void {
    this.loading.set(true);
    this.error.set(null);
    this.clear();

    forkJoin({
      allProducts: this.products.list({ limit: 1, offset: 0 }),
      lowStock: this.products.list({ status: 'low', limit: 5, offset: 0 }),
      emptyStock: this.products.list({ status: 'empty', limit: 5, offset: 0 }),
      openInvoices: this.billing.list({ status: 'OPEN', limit: 1, offset: 0 }),
      closedInvoices: this.billing.list({ status: 'CLOSED', limit: 1, offset: 0 }),
      recentInvoices: this.billing.list({ limit: 5, offset: 0 }),
    })
      .pipe(
        catchError((error: ApiError) => {
          this.error.set(error);
          this.clear();
          return EMPTY;
        }),
        finalize(() => this.loading.set(false)),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe((result) => {
        this.totalProducts.set(result.allProducts.total);
        this.attentionStockTotal.set(result.lowStock.total);
        this.openInvoicesTotal.set(result.openInvoices.total);
        this.closedInvoicesTotal.set(result.closedInvoices.total);
        this.recentInvoices.set(result.recentInvoices.items);
        this.criticalProducts.set(
          [...result.emptyStock.items, ...result.lowStock.items]
            .sort((first, second) => first.balance - second.balance)
            .slice(0, 5),
        );
      });
  }

  private clear(): void {
    this.totalProducts.set(0);
    this.attentionStockTotal.set(0);
    this.openInvoicesTotal.set(0);
    this.closedInvoicesTotal.set(0);
    this.recentInvoices.set([]);
    this.criticalProducts.set([]);
  }
}
