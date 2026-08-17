import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  OnInit,
  signal,
  untracked,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatTooltipModule } from '@angular/material/tooltip';
import { RouterLink } from '@angular/router';
import { debounceTime, distinctUntilChanged, Subject } from 'rxjs';

import { ErrorBannerComponent } from '../../../../shared/ui/error-banner.component';
import { AppIconComponent } from '../../../../shared/ui/app-icon.component';
import { LoadingStateComponent } from '../../../../shared/ui/loading-state.component';
import { PaginationComponent } from '../../../../shared/ui/pagination.component';
import { ProductStore } from '../../application/product.store';
import { ProductStockFilter } from '../../domain/product';

@Component({
  imports: [
    ErrorBannerComponent,
    AppIconComponent,
    LoadingStateComponent,
    MatProgressSpinnerModule,
    MatTooltipModule,
    PaginationComponent,
    RouterLink,
  ],
  providers: [ProductStore],
  templateUrl: './product-list.component.html',
  styleUrl: './product-list.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProductListComponent implements OnInit {
  protected readonly store = inject(ProductStore);
  protected readonly search = signal('');
  protected readonly filter = signal<ProductStockFilter>('all');
  protected readonly currentPage = signal(1);
  protected readonly pageSize = 10;
  protected readonly hasRemoteFilters = computed(
    () => this.search().trim().length > 0 || this.filter() !== 'all',
  );
  private readonly searchChanges = new Subject<string>();

  constructor() {
    this.searchChanges
      .pipe(debounceTime(300), distinctUntilChanged(), takeUntilDestroyed())
      .subscribe(() => this.loadProducts());
    effect(() => {
      const currentPage = this.currentPage();
      const total = this.store.total();
      if (!this.store.loaded() || this.store.loading() || this.store.error()) return;
      const lastPage = Math.max(1, Math.ceil(total / this.pageSize));
      if (currentPage <= lastPage) return;
      this.currentPage.set(lastPage);
      untracked(() => this.loadProducts());
    });
  }

  ngOnInit(): void {
    this.loadProducts();
  }

  protected updateSearch(event: Event): void {
    const value = (event.target as HTMLInputElement).value;
    this.search.set(value);
    this.currentPage.set(1);
    this.searchChanges.next(value.trim());
  }

  protected setFilter(filter: ProductStockFilter): void {
    if (filter === this.filter()) return;
    this.filter.set(filter);
    this.currentPage.set(1);
    this.loadProducts();
  }

  protected goToPage(page: number): void {
    if (page === this.currentPage()) return;
    this.currentPage.set(page);
    this.loadProducts();
  }

  protected loadProducts(): void {
    this.store.load({
      search: this.search().trim() || undefined,
      status: this.filter(),
      limit: this.pageSize,
      offset: (this.currentPage() - 1) * this.pageSize,
    });
  }

  protected stockStatus(balance: number): 'active' | 'low' | 'empty' {
    if (balance === 0) return 'empty';
    return balance <= 5 ? 'low' : 'active';
  }
}
