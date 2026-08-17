import { ChangeDetectionStrategy, Component, inject, OnInit } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ReactiveFormsModule } from '@angular/forms';
import { MatAutocompleteModule } from '@angular/material/autocomplete';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { Router, RouterLink } from '@angular/router';

import { ErrorBannerComponent } from '../../../../shared/ui/error-banner.component';
import { AppIconComponent } from '../../../../shared/ui/app-icon.component';
import { LoadingStateComponent } from '../../../../shared/ui/loading-state.component';
import { InvoiceFormFacade } from '../../application/invoice-form.facade';

export {
  INVOICE_CREATE_IDEMPOTENCY_STORAGE_KEY,
  MAXIMUM_INVOICE_ITEMS,
  maximumItemsValidator,
  selectedProductValidator,
  sufficientStockValidator,
  uniqueProductsValidator,
} from '../../application/invoice-form.facade';

@Component({
  imports: [
    ErrorBannerComponent,
    AppIconComponent,
    LoadingStateComponent,
    MatAutocompleteModule,
    MatButtonModule,
    MatFormFieldModule,
    MatInputModule,
    MatProgressSpinnerModule,
    ReactiveFormsModule,
    RouterLink,
  ],
  templateUrl: './invoice-form.component.html',
  styleUrl: './invoice-form.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  providers: [InvoiceFormFacade],
})
export class InvoiceFormComponent implements OnInit {
  private readonly facade = inject(InvoiceFormFacade);
  private readonly router = inject(Router);

  protected readonly products = this.facade.products;
  protected readonly loadingProducts = this.facade.loadingProducts;
  protected readonly submitting = this.facade.submitting;
  protected readonly error = this.facade.error;
  protected readonly errorTitle = this.facade.errorTitle;
  protected readonly maximumInvoiceItems = this.facade.maximumInvoiceItems;
  protected readonly form = this.facade.form;
  protected readonly productDisplay = this.facade.productDisplay;

  protected get items() {
    return this.facade.items;
  }

  constructor() {
    this.facade.created$
      .pipe(takeUntilDestroyed())
      .subscribe((invoice) => void this.router.navigate(['/notas', invoice.id]));
  }

  ngOnInit(): void {
    this.facade.loadProducts();
  }

  protected addItem(): void {
    this.facade.addItem();
  }

  protected removeItem(index: number): void {
    this.facade.removeItem(index);
  }

  protected productBalance(productId: string): number | undefined {
    return this.facade.productBalance(productId);
  }

  protected productOptions = this.facade.productOptions.bind(this.facade);
  protected isSearchingProducts = this.facade.isSearchingProducts.bind(this.facade);
  protected selectedItems = this.facade.selectedItems.bind(this.facade);
  protected totalUnits = this.facade.totalUnits.bind(this.facade);

  protected submit(): void {
    this.facade.submit();
  }

  protected retryLastAction(): void {
    this.facade.retryLastAction();
  }
}
