import {
  ChangeDetectionStrategy,
  Component,
  effect,
  inject,
  OnInit,
  signal,
  untracked,
} from '@angular/core';
import { DatePipe } from '@angular/common';
import { MatButtonModule } from '@angular/material/button';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatTableModule } from '@angular/material/table';
import { RouterLink } from '@angular/router';

import { ErrorBannerComponent } from '../../../../shared/ui/error-banner.component';
import { AppIconComponent } from '../../../../shared/ui/app-icon.component';
import { LoadingStateComponent } from '../../../../shared/ui/loading-state.component';
import { PaginationComponent } from '../../../../shared/ui/pagination.component';
import { InvoiceStore } from '../../application/invoice.store';
import { InvoiceListFilter } from '../../domain/invoice';
import { InvoiceStatusBadgeComponent } from '../shared/invoice-status-badge.component';
import { InvoiceTotalUnitsPipe } from '../shared/invoice-total-units.pipe';

@Component({
  imports: [
    AppIconComponent,
    DatePipe,
    ErrorBannerComponent,
    InvoiceStatusBadgeComponent,
    InvoiceTotalUnitsPipe,
    LoadingStateComponent,
    MatButtonModule,
    MatProgressSpinnerModule,
    MatTableModule,
    PaginationComponent,
    RouterLink,
  ],
  templateUrl: './invoice-list.component.html',
  styleUrl: './invoice-list.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  providers: [InvoiceStore],
})
export class InvoiceListComponent implements OnInit {
  protected readonly store = inject(InvoiceStore);
  protected readonly displayedColumns = ['number', 'createdAt', 'items', 'status', 'actions'];
  protected readonly filter = signal<InvoiceListFilter>('all');
  protected readonly currentPage = signal(1);
  protected readonly pageSize = 10;

  constructor() {
    effect(() => {
      const currentPage = this.currentPage();
      const total = this.store.total();
      if (!this.store.loaded() || this.store.loading() || this.store.error()) return;
      const lastPage = Math.max(1, Math.ceil(total / this.pageSize));
      if (currentPage <= lastPage) return;
      this.currentPage.set(lastPage);
      untracked(() => this.loadInvoices());
    });
  }

  ngOnInit(): void {
    this.loadInvoices();
  }

  protected setFilter(filter: InvoiceListFilter): void {
    if (filter === this.filter()) return;
    this.filter.set(filter);
    this.currentPage.set(1);
    this.loadInvoices();
  }

  protected goToPage(page: number): void {
    if (page === this.currentPage()) return;
    this.currentPage.set(page);
    this.loadInvoices();
  }

  protected loadInvoices(): void {
    const filter = this.filter();
    this.store.load({
      status: filter === 'all' ? undefined : filter,
      limit: this.pageSize,
      offset: (this.currentPage() - 1) * this.pageSize,
    });
  }
}
