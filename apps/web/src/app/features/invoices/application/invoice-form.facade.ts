import { computed, DestroyRef, inject, Injectable, signal, WritableSignal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import {
  AbstractControl,
  FormArray,
  FormControl,
  FormGroup,
  ValidationErrors,
  ValidatorFn,
  Validators,
} from '@angular/forms';
import {
  catchError,
  debounceTime,
  distinctUntilChanged,
  EMPTY,
  finalize,
  Observable,
  Subject,
  switchMap,
} from 'rxjs';

import { IdempotencyStore } from '../../../core/application/idempotency-store';
import { ApiError } from '../../../core/errors/api-error';
import { ProductGateway } from '../../products/application/product.gateway';
import { Product, ProductCollection } from '../../products/domain/product';
import { CreateInvoiceRequest, Invoice } from '../domain/invoice';
import { BillingGateway } from './billing.gateway';

export const INVOICE_CREATE_IDEMPOTENCY_STORAGE_KEY = 'invoice-create:idempotency';
export const MAXIMUM_INVOICE_ITEMS = 20;

type InvoiceFormErrorSource = 'catalog-load' | 'catalog-search' | 'submission';

const productDescriptionCollator = new Intl.Collator('pt-BR', {
  numeric: true,
  sensitivity: 'base',
});

interface FailedCatalogSearch {
  readonly control: FormControl<string>;
  readonly term: string;
}

export type InvoiceItemForm = FormGroup<{
  productId: FormControl<string>;
  quantity: FormControl<number>;
}>;

export const uniqueProductsValidator: ValidatorFn = (
  control: AbstractControl,
): ValidationErrors | null => {
  if (!(control instanceof FormArray)) return null;
  const selected = control.controls
    .map((item) => item.get('productId')?.value)
    .filter((value): value is string => typeof value === 'string' && value.length > 0);
  return new Set(selected).size === selected.length ? null : { duplicateProducts: true };
};

export function maximumItemsValidator(maximum: number): ValidatorFn {
  return (control: AbstractControl): ValidationErrors | null => {
    if (!(control instanceof FormArray) || control.length <= maximum) return null;
    return { maximumItems: { maximum, actual: control.length } };
  };
}

export function selectedProductValidator(
  findProduct: (productId: string) => Product | undefined,
): ValidatorFn {
  return (control: AbstractControl): ValidationErrors | null => {
    const productId = typeof control.value === 'string' ? control.value.trim() : '';
    return productId === '' || findProduct(productId) ? null : { productNotSelected: true };
  };
}

export function sufficientStockValidator(
  findProduct: (productId: string) => Product | undefined,
): ValidatorFn {
  return (control: AbstractControl): ValidationErrors | null => {
    const productId = control.get('productId')?.value;
    const quantity = control.get('quantity')?.value;
    if (typeof productId !== 'string' || typeof quantity !== 'number') return null;

    const product = findProduct(productId);
    return product && quantity > product.balance
      ? { insufficientStock: { available: product.balance } }
      : null;
  };
}

@Injectable()
export class InvoiceFormFacade {
  private readonly billing = inject(BillingGateway);
  private readonly destroyRef = inject(DestroyRef);
  private readonly identities = inject(IdempotencyStore);
  private readonly productGateway = inject(ProductGateway);
  private readonly optionsByControl = new WeakMap<
    FormControl<string>,
    WritableSignal<readonly Product[]>
  >();
  private readonly searchingByControl = new WeakMap<FormControl<string>, WritableSignal<boolean>>();
  private readonly createdSubject = new Subject<Invoice>();

  readonly created$ = this.createdSubject.asObservable();
  readonly products = signal<readonly Product[]>([]);
  readonly loadingProducts = signal(false);
  readonly submitting = signal(false);
  readonly error = signal<ApiError | null>(null);
  private readonly errorSource = signal<InvoiceFormErrorSource | null>(null);
  readonly errorTitle = computed(() => {
    switch (this.errorSource()) {
      case 'submission':
        return 'Não foi possível criar a nota';
      case 'catalog-search':
        return 'Não foi possível pesquisar os produtos';
      default:
        return 'Não foi possível carregar o catálogo de produtos';
    }
  });
  readonly maximumInvoiceItems = MAXIMUM_INVOICE_ITEMS;
  private failedCatalogSearch: FailedCatalogSearch | null = null;
  private readonly requireCatalogProduct = selectedProductValidator((id) => this.findProduct(id));
  private readonly requireSufficientStock = sufficientStockValidator((id) => this.findProduct(id));
  readonly form = new FormGroup({
    items: new FormArray<InvoiceItemForm>([this.createItemForm()], {
      validators: [
        Validators.required,
        uniqueProductsValidator,
        maximumItemsValidator(MAXIMUM_INVOICE_ITEMS),
      ],
    }),
  });

  get items(): FormArray<InvoiceItemForm> {
    return this.form.controls.items;
  }

  loadProducts(): void {
    const firstControl = this.items.at(0).controls.productId;
    this.loadingProducts.set(true);
    this.searchingSignal(firstControl).set(true);
    this.resetError();
    this.productGateway
      .list({ limit: 20, offset: 0 })
      .pipe(
        catchError((error: ApiError) => {
          this.error.set(error);
          this.errorSource.set('catalog-load');
          return EMPTY;
        }),
        finalize(() => {
          this.loadingProducts.set(false);
          this.searchingSignal(firstControl).set(false);
        }),
      )
      .subscribe((collection) => {
        const orderedProducts = this.orderProductOptions(collection.items);
        this.mergeProducts(orderedProducts);
        this.optionsSignal(firstControl).set(orderedProducts);
      });
  }

  addItem(): void {
    if (this.items.length < MAXIMUM_INVOICE_ITEMS) this.items.push(this.createItemForm());
  }

  removeItem(index: number): void {
    if (this.items.length > 1) this.items.removeAt(index);
  }

  productBalance(productId: string): number | undefined {
    return this.findProduct(productId)?.balance;
  }

  productOptions(control: FormControl<string>): readonly Product[] {
    return this.optionsSignal(control)();
  }

  isSearchingProducts(control: FormControl<string>): boolean {
    return this.searchingSignal(control)();
  }

  readonly productDisplay = (productId: string): string =>
    this.findProduct(productId)?.description ?? productId;

  selectedItems(): readonly { product: Product; quantity: number }[] {
    return this.items.controls.flatMap((item) => {
      const product = this.findProduct(item.controls.productId.value);
      return product ? [{ product, quantity: item.controls.quantity.value }] : [];
    });
  }

  totalUnits(): number {
    return this.selectedItems().reduce((total, item) => total + item.quantity, 0);
  }

  submit(): void {
    if (this.form.invalid || this.submitting()) {
      this.form.markAllAsTouched();
      return;
    }

    this.submitting.set(true);
    this.resetError();
    const request: CreateInvoiceRequest = { items: this.items.getRawValue() };
    const fingerprint = JSON.stringify(request);
    const idempotencyKey = this.identities.getOrCreate(
      INVOICE_CREATE_IDEMPOTENCY_STORAGE_KEY,
      fingerprint,
    );
    this.billing
      .create(request, idempotencyKey)
      .pipe(
        catchError((error: ApiError) => {
          this.error.set(error);
          this.errorSource.set('submission');
          if (!error.retryable) this.clearCreateIdentity();
          return EMPTY;
        }),
        finalize(() => this.submitting.set(false)),
      )
      .subscribe((invoice) => {
        this.clearCreateIdentity();
        this.createdSubject.next(invoice);
      });
  }

  retryLastAction(): void {
    if (this.errorSource() === 'submission') {
      this.submit();
      return;
    }

    const failedSearch = this.failedCatalogSearch;
    if (
      failedSearch &&
      this.items.controls.some((item) => item.controls.productId === failedSearch.control)
    ) {
      this.searchProducts(failedSearch.control, failedSearch.term)
        .pipe(takeUntilDestroyed(this.destroyRef))
        .subscribe((collection) => this.applySearchResult(failedSearch.control, collection));
      return;
    }
    this.loadProducts();
  }

  private createItemForm(): InvoiceItemForm {
    const productId = new FormControl('', {
      nonNullable: true,
      validators: [Validators.required, this.requireCatalogProduct],
    });
    const form = new FormGroup(
      {
        productId,
        quantity: new FormControl(1, {
          nonNullable: true,
          validators: [
            Validators.required,
            Validators.min(1),
            Validators.max(Number.MAX_SAFE_INTEGER),
            Validators.pattern(/^\d+$/),
          ],
        }),
      },
      { validators: this.requireSufficientStock },
    );

    this.optionsSignal(productId).set(this.products());
    this.watchProductSearch(productId);
    return form;
  }

  private watchProductSearch(control: FormControl<string>): void {
    control.valueChanges
      .pipe(
        debounceTime(300),
        distinctUntilChanged(),
        switchMap((value) => {
          const term = value.trim();
          return this.findProduct(term) ? EMPTY : this.searchProducts(control, term);
        }),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe((collection) => this.applySearchResult(control, collection));
  }

  private searchProducts(
    control: FormControl<string>,
    term: string,
  ): Observable<ProductCollection> {
    this.searchingSignal(control).set(true);
    this.resetError();
    return this.productGateway.list({ search: term || undefined, limit: 20, offset: 0 }).pipe(
      catchError((error: ApiError) => {
        this.error.set(error);
        this.errorSource.set('catalog-search');
        this.failedCatalogSearch = { control, term };
        return EMPTY;
      }),
      finalize(() => this.searchingSignal(control).set(false)),
    );
  }

  private applySearchResult(control: FormControl<string>, collection: ProductCollection): void {
    const orderedProducts = this.orderProductOptions(collection.items);
    this.mergeProducts(orderedProducts);
    this.optionsSignal(control).set(orderedProducts);
  }

  private mergeProducts(incoming: readonly Product[]): void {
    this.products.update((current) => {
      const merged = new Map(current.map((product) => [product.id, product]));
      for (const product of incoming) merged.set(product.id, product);
      return this.orderProductOptions([...merged.values()]);
    });
    for (const item of this.items.controls) {
      item.controls.productId.updateValueAndValidity({ emitEvent: false });
      item.updateValueAndValidity({ emitEvent: false });
    }
  }

  private findProduct(productId: string): Product | undefined {
    return this.products().find((product) => product.id === productId);
  }

  private orderProductOptions(products: readonly Product[]): readonly Product[] {
    return [...products].sort(
      (left, right) =>
        productDescriptionCollator.compare(left.description, right.description) ||
        productDescriptionCollator.compare(left.code, right.code),
    );
  }

  private clearCreateIdentity(): void {
    this.identities.clear(INVOICE_CREATE_IDEMPOTENCY_STORAGE_KEY);
  }

  private resetError(): void {
    this.error.set(null);
    this.errorSource.set(null);
    this.failedCatalogSearch = null;
  }

  private optionsSignal(control: FormControl<string>): WritableSignal<readonly Product[]> {
    let options = this.optionsByControl.get(control);
    if (!options) {
      options = signal<readonly Product[]>([]);
      this.optionsByControl.set(control, options);
    }
    return options;
  }

  private searchingSignal(control: FormControl<string>): WritableSignal<boolean> {
    let searching = this.searchingByControl.get(control);
    if (!searching) {
      searching = signal(false);
      this.searchingByControl.set(control, searching);
    }
    return searching;
  }
}
