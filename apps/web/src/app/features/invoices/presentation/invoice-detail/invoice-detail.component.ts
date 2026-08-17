import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, inject, OnInit } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MatButtonModule } from '@angular/material/button';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';

import { ErrorBannerComponent } from '../../../../shared/ui/error-banner.component';
import { AppIconComponent } from '../../../../shared/ui/app-icon.component';
import { LoadingStateComponent } from '../../../../shared/ui/loading-state.component';
import { InvoiceDetailFacade } from '../../application/invoice-detail.facade';
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
    RouterLink,
  ],
  templateUrl: './invoice-detail.component.html',
  styleUrl: './invoice-detail.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  providers: [InvoiceDetailFacade],
})
export class InvoiceDetailComponent implements OnInit {
  private readonly facade = inject(InvoiceDetailFacade);
  private readonly invoiceId = inject(ActivatedRoute).snapshot.paramMap.get('id') ?? '';
  private readonly router = inject(Router);

  protected readonly invoice = this.facade.invoice;
  protected readonly operation = this.facade.operation;
  protected readonly loading = this.facade.loading;
  protected readonly processing = this.facade.processing;
  protected readonly error = this.facade.error;
  protected readonly errorTitle = this.facade.errorTitle;
  protected readonly operationLabel = this.facade.operationLabel.bind(this.facade);
  protected readonly operationProgress = this.facade.operationProgress.bind(this.facade);
  protected readonly stepState = this.facade.stepState.bind(this.facade);

  constructor() {
    this.facade.printRequested$
      .pipe(takeUntilDestroyed())
      .subscribe((id) => void this.router.navigate(['/notas', id, 'impressao']));
  }

  ngOnInit(): void {
    this.facade.initialize(this.invoiceId);
  }

  protected loadInvoice(): void {
    this.facade.loadInvoice();
  }

  protected requestClose(): void {
    this.facade.requestClose();
  }

  protected retryLastAction(): void {
    this.facade.retryLastAction();
  }
}
