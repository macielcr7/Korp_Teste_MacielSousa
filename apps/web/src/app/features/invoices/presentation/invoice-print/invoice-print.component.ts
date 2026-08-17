import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, inject, OnInit } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { ActivatedRoute, RouterLink } from '@angular/router';

import { ErrorBannerComponent } from '../../../../shared/ui/error-banner.component';
import { AppIconComponent } from '../../../../shared/ui/app-icon.component';
import { LoadingStateComponent } from '../../../../shared/ui/loading-state.component';
import { InvoicePrintFacade } from '../../application/invoice-print.facade';
import { InvoiceTotalUnitsPipe } from '../shared/invoice-total-units.pipe';

@Component({
  imports: [
    AppIconComponent,
    DatePipe,
    ErrorBannerComponent,
    InvoiceTotalUnitsPipe,
    LoadingStateComponent,
    MatButtonModule,
    RouterLink,
  ],
  templateUrl: './invoice-print.component.html',
  styleUrl: './invoice-print.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  providers: [InvoicePrintFacade],
})
export class InvoicePrintComponent implements OnInit {
  private readonly facade = inject(InvoicePrintFacade);
  protected readonly invoiceId = inject(ActivatedRoute).snapshot.paramMap.get('id') ?? '';

  protected readonly invoice = this.facade.invoice;
  protected readonly loading = this.facade.loading;
  protected readonly error = this.facade.error;

  ngOnInit(): void {
    this.loadInvoice();
  }

  protected loadInvoice(): void {
    this.facade.load(this.invoiceId);
  }

  protected print(): void {
    window.print();
  }
}
