import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, inject, OnInit } from '@angular/core';
import { RouterLink } from '@angular/router';

import { AppIconComponent } from '../../shared/ui/app-icon.component';
import { ErrorBannerComponent } from '../../shared/ui/error-banner.component';
import { InvoiceStatusBadgeComponent } from '../invoices/presentation/shared/invoice-status-badge.component';
import { DashboardFacade } from './dashboard.facade';

@Component({
  imports: [AppIconComponent, DatePipe, ErrorBannerComponent, InvoiceStatusBadgeComponent, RouterLink],
  templateUrl: './dashboard.component.html',
  styleUrl: './dashboard.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  providers: [DashboardFacade],
})
export class DashboardComponent implements OnInit {
  private readonly facade = inject(DashboardFacade);
  protected readonly today = this.facade.today;
  protected readonly totalProducts = this.facade.totalProducts;
  protected readonly attentionStockTotal = this.facade.attentionStockTotal;
  protected readonly openInvoicesTotal = this.facade.openInvoicesTotal;
  protected readonly closedInvoicesTotal = this.facade.closedInvoicesTotal;
  protected readonly recentInvoices = this.facade.recentInvoices;
  protected readonly criticalProducts = this.facade.criticalProducts;
  protected readonly loading = this.facade.loading;
  protected readonly error = this.facade.error;

  ngOnInit(): void {
    this.reload();
  }

  protected reload(): void {
    this.facade.reload();
  }
}
