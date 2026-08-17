import { FormArray, FormControl, FormGroup } from '@angular/forms';
import { TestBed } from '@angular/core/testing';
import { provideRouter, Router } from '@angular/router';
import { Observable, of, throwError } from 'rxjs';

import { IdempotencyStore } from '../../../../core/application/idempotency-store';
import { ApiError } from '../../../../core/errors/api-error';
import { SessionIdempotencyStore } from '../../../../core/infrastructure/session-idempotency-store';
import { ProductGateway } from '../../../products/application/product.gateway';
import { Product } from '../../../products/domain/product';
import { BillingGateway } from '../../application/billing.gateway';
import { CreateInvoiceRequest, Invoice } from '../../domain/invoice';
import {
  INVOICE_CREATE_IDEMPOTENCY_STORAGE_KEY,
  InvoiceFormComponent,
  MAXIMUM_INVOICE_ITEMS,
  maximumItemsValidator,
  selectedProductValidator,
  sufficientStockValidator,
  uniqueProductsValidator,
} from './invoice-form.component';

describe('uniqueProductsValidator', () => {
  function item(productId: string): FormGroup {
    return new FormGroup({ productId: new FormControl(productId) });
  }

  it('rejects duplicate products', () => {
    const items = new FormArray([item('product-1'), item('product-1')]);

    expect(uniqueProductsValidator(items)).toEqual({ duplicateProducts: true });
  });

  it('accepts distinct products', () => {
    const items = new FormArray([item('product-1'), item('product-2')]);

    expect(uniqueProductsValidator(items)).toBeNull();
  });

  it('rejects an item collection above the configured maximum', () => {
    const items = new FormArray([item('product-1'), item('product-2')]);

    expect(maximumItemsValidator(1)(items)).toEqual({
      maximumItems: { maximum: 1, actual: 2 },
    });
  });
});

describe('invoice item validators', () => {
  const product: Product = {
    id: 'product-1',
    code: 'SKU-001',
    description: 'Produto de teste',
    balance: 5,
  };
  const findProduct = (productId: string): Product | undefined =>
    productId === product.id ? product : undefined;

  it('requires a product selected from the remote search results', () => {
    const control = new FormControl('texto digitado');

    expect(selectedProductValidator(findProduct)(control)).toEqual({ productNotSelected: true });
    control.setValue(product.id);
    expect(selectedProductValidator(findProduct)(control)).toBeNull();
  });

  it('rejects a quantity above the available balance', () => {
    const item = new FormGroup({
      productId: new FormControl(product.id),
      quantity: new FormControl(6),
    });

    expect(sufficientStockValidator(findProduct)(item)).toEqual({
      insufficientStock: { available: 5 },
    });
  });
});

describe('InvoiceFormComponent idempotency', () => {
  const product: Product = {
    id: 'product-1',
    code: 'PRD-001',
    description: 'Produto de teste',
    balance: 5,
  };
  const createdInvoice: Invoice = {
    id: 'invoice-1',
    number: 1,
    status: 'OPEN',
    createdAt: '2026-08-13T10:00:00Z',
    items: [],
  };

  afterEach(() => {
    sessionStorage.removeItem(INVOICE_CREATE_IDEMPOTENCY_STORAGE_KEY);
    vi.useRealTimers();
  });

  it('preserves the creation key after a transient failure and reuses it for the same payload', () => {
    const transientError = new ApiError('Serviço indisponível.', 503, undefined, undefined, true);
    const { billingApi, submit } = createFixture(throwError(() => transientError));

    submit();
    const firstKey = billingApi.create.mock.calls[0][1];
    expect(sessionStorage.getItem(INVOICE_CREATE_IDEMPOTENCY_STORAGE_KEY)).toContain(firstKey);

    submit();
    expect(billingApi.create.mock.calls[1][1]).toBe(firstKey);
  });

  it('retries a transient submission instead of reloading the catalog', () => {
    const transientError = new ApiError('Serviço indisponível.', 503, undefined, undefined, true);
    const { billingApi, fixture, productApi, submit } = createFixture(
      throwError(() => transientError),
    );

    submit();
    fixture.detectChanges();
    const element = fixture.nativeElement as HTMLElement;
    expect(element.textContent).toContain('Não foi possível criar a nota');
    const retryButton = [...element.querySelectorAll<HTMLButtonElement>('button')].find((button) =>
      button.textContent?.includes('Tentar novamente'),
    );
    retryButton?.click();

    expect(retryButton).toBeDefined();
    expect(billingApi.create).toHaveBeenCalledTimes(2);
    expect(billingApi.create.mock.calls[1][1]).toBe(billingApi.create.mock.calls[0][1]);
    expect(productApi.list).toHaveBeenCalledTimes(1);
  });

  it('retries the failed remote product search for the same item and term', async () => {
    vi.useFakeTimers();
    const transientError = new ApiError('Serviço indisponível.', 503, undefined, undefined, true);
    const remoteProduct: Product = {
      id: 'product-2',
      code: 'PRD-002',
      description: 'Produto remoto',
      balance: 8,
    };
    const { component, fixture, productApi } = createFixture(of(createdInvoice));
    component.addItem();
    const secondProductControl = component.items.at(1).get('productId') as FormControl<string>;
    productApi.list.mockReturnValueOnce(throwError(() => transientError));

    secondProductControl.setValue('Produto remoto');
    await vi.advanceTimersByTimeAsync(300);
    fixture.detectChanges();
    productApi.list.mockReturnValueOnce(
      of({ items: [remoteProduct], total: 1, limit: 20, offset: 0 }),
    );

    const element = fixture.nativeElement as HTMLElement;
    expect(element.textContent).toContain('Não foi possível pesquisar os produtos');
    const retryButton = [...element.querySelectorAll<HTMLButtonElement>('button')].find((button) =>
      button.textContent?.includes('Tentar novamente'),
    );
    retryButton?.click();

    expect(retryButton).toBeDefined();
    expect(productApi.list).toHaveBeenLastCalledWith({
      search: 'Produto remoto',
      limit: 20,
      offset: 0,
    });
    expect(component.productOptions(secondProductControl)).toEqual([remoteProduct]);
  });

  it('orders autocomplete options alphabetically by product description', () => {
    const webcam: Product = {
      id: 'product-webcam',
      code: 'PRD-003',
      description: 'Webcam Full HD',
      balance: 6,
    };
    const cable: Product = {
      id: 'product-cable',
      code: 'PRD-005',
      description: 'Cabo HDMI 2 metros',
      balance: 20,
    };
    const { component } = createFixture(of(createdInvoice), [webcam, product, cable]);
    const firstProductControl = component.items.at(0).get('productId') as FormControl<string>;

    expect(component.productOptions(firstProductControl)).toEqual([cable, product, webcam]);
  });

  it('clears the creation key after a terminal failure', () => {
    const terminalError = new ApiError('Dados inválidos.', 422, undefined, undefined, false);
    const { submit } = createFixture(throwError(() => terminalError));

    submit();

    expect(sessionStorage.getItem(INVOICE_CREATE_IDEMPOTENCY_STORAGE_KEY)).toBeNull();
  });

  it('does not offer a retry for a terminal submission error', () => {
    const terminalError = new ApiError('Dados inválidos.', 422, undefined, undefined, false);
    const { fixture, submit } = createFixture(throwError(() => terminalError));

    submit();
    fixture.detectChanges();

    expect(fixture.nativeElement.textContent).toContain('Não foi possível criar a nota');
    expect(fixture.nativeElement.textContent).not.toContain('Tentar novamente');
  });

  it('clears the creation key after success', () => {
    const { submit } = createFixture(of(createdInvoice));

    submit();

    expect(sessionStorage.getItem(INVOICE_CREATE_IDEMPOTENCY_STORAGE_KEY)).toBeNull();
    expect(TestBed.inject(Router).navigate).toHaveBeenCalledWith(['/notas', createdInvoice.id]);
  });

  it('blocks additions after reaching the API item limit', () => {
    const { component, fixture } = createFixture(of(createdInvoice));

    for (let index = component.items.length; index < MAXIMUM_INVOICE_ITEMS + 5; index += 1) {
      component.addItem();
    }
    fixture.detectChanges();

    expect(component.items.length).toBe(MAXIMUM_INVOICE_ITEMS);
    const element = fixture.nativeElement as HTMLElement;
    const addButton = [...element.querySelectorAll<HTMLButtonElement>('button')].find((button) =>
      button.textContent?.includes('Adicionar item'),
    );
    expect(addButton?.disabled).toBe(true);
    fixture.destroy();
  });

  function createFixture(
    createResult: Observable<Invoice>,
    catalog: readonly Product[] = [product],
  ) {
    const billingApi = {
      create: vi.fn<(request: CreateInvoiceRequest, idempotencyKey: string) => Observable<Invoice>>(
        () => createResult,
      ),
    };
    const productApi = {
      list: vi.fn(() => of({ items: catalog, total: catalog.length, limit: 20, offset: 0 })),
    };
    TestBed.configureTestingModule({
      imports: [InvoiceFormComponent],
      providers: [
        provideRouter([]),
        { provide: BillingGateway, useValue: billingApi },
        { provide: ProductGateway, useValue: productApi },
        { provide: IdempotencyStore, useClass: SessionIdempotencyStore },
      ],
    });
    const router = TestBed.inject(Router);
    vi.spyOn(router, 'navigate').mockResolvedValue(true);
    const fixture = TestBed.createComponent(InvoiceFormComponent);
    fixture.detectChanges();
    const component = fixture.componentInstance as unknown as {
      readonly form: FormGroup;
      readonly items: FormArray;
      addItem(): void;
      productOptions(control: FormControl<string>): readonly Product[];
      submit(): void;
    };
    component.form.get('items.0.productId')?.setValue(product.id);
    component.form.get('items.0.quantity')?.setValue(2);

    return { billingApi, component, fixture, productApi, submit: () => component.submit() };
  }
});
