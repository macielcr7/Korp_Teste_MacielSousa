import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { MatTooltip } from '@angular/material/tooltip';
import { By } from '@angular/platform-browser';
import { provideRouter } from '@angular/router';

import { ApiError } from '../../../../core/errors/api-error';
import { ProductStore } from '../../application/product.store';
import { Product } from '../../domain/product';
import { ProductListComponent } from './product-list.component';

describe('ProductListComponent', () => {
  afterEach(() => vi.useRealTimers());

  it('requests ten products per page and performs debounced remote search', async () => {
    vi.useFakeTimers();
    const { fixture, store } = createFixture();
    fixture.detectChanges();

    expect(store.load).toHaveBeenCalledWith({
      search: undefined,
      status: 'all',
      limit: 10,
      offset: 0,
    });

    const root = fixture.nativeElement as HTMLElement;
    const search = root.querySelector<HTMLInputElement>('input[type="search"]');
    if (!search) throw new Error('Product search input was not rendered.');
    search.value = 'teclado';
    search.dispatchEvent(new Event('input'));

    expect(store.load).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(300);
    expect(store.load).toHaveBeenLastCalledWith({
      search: 'teclado',
      status: 'all',
      limit: 10,
      offset: 0,
    });
    fixture.destroy();
  });

  it('requests status and page filters remotely', () => {
    const { fixture, store } = createFixture(25);
    fixture.detectChanges();

    const root = fixture.nativeElement as HTMLElement;
    const lowFilter = Array.from(
      root.querySelectorAll<HTMLButtonElement>('.filter-tabs button'),
    ).find((button) => button.textContent?.trim() === 'Saldo Baixo');
    lowFilter?.click();
    expect(store.load).toHaveBeenLastCalledWith({
      search: undefined,
      status: 'low',
      limit: 10,
      offset: 0,
    });

    const nextPage = root.querySelector<HTMLButtonElement>('button[aria-label="Próxima página"]');
    nextPage?.click();
    expect(store.load).toHaveBeenLastCalledWith({
      search: undefined,
      status: 'low',
      limit: 10,
      offset: 10,
    });
    fixture.destroy();
  });

  it('explains unavailable row actions with tooltips', () => {
    const { fixture } = createFixture(1);
    fixture.detectChanges();

    const messages = fixture.debugElement
      .queryAll(By.directive(MatTooltip))
      .map((element) => element.injector.get(MatTooltip).message);

    expect(messages).toEqual([
      'Visualização ainda não disponível',
      'Edição ainda não disponível',
      'Exclusão ainda não disponível',
    ]);
    const actions = Array.from(
      (fixture.nativeElement as HTMLElement).querySelectorAll<HTMLElement>(
        '.row-actions .unavailable-action',
      ),
    );
    expect(actions).toHaveLength(3);
    expect(
      actions.map((action) => ({
        role: action.getAttribute('role'),
        tabIndex: action.tabIndex,
        disabled: action.getAttribute('aria-disabled'),
        label: action.getAttribute('aria-label'),
      })),
    ).toEqual([
      {
        role: 'button',
        tabIndex: 0,
        disabled: 'true',
        label: 'Visualizar SKU-001 — ação indisponível',
      },
      {
        role: 'button',
        tabIndex: 0,
        disabled: 'true',
        label: 'Editar SKU-001 — ação indisponível',
      },
      {
        role: 'button',
        tabIndex: 0,
        disabled: 'true',
        label: 'Excluir SKU-001 — ação indisponível',
      },
    ]);
    fixture.destroy();
  });

  it('reloads the last valid remote page when the total shrinks', async () => {
    const { fixture, store } = createFixture(25);
    fixture.detectChanges();
    const root = fixture.nativeElement as HTMLElement;

    root.querySelector<HTMLButtonElement>('button[aria-label="Próxima página"]')?.click();
    fixture.detectChanges();
    root.querySelector<HTMLButtonElement>('button[aria-label="Próxima página"]')?.click();
    expect(store.load).toHaveBeenLastCalledWith({
      search: undefined,
      status: 'all',
      limit: 10,
      offset: 20,
    });

    store.total.set(15);
    fixture.detectChanges();
    await fixture.whenStable();

    expect(store.load).toHaveBeenLastCalledWith({
      search: undefined,
      status: 'all',
      limit: 10,
      offset: 10,
    });
    fixture.destroy();
  });

  function createFixture(total = 1) {
    const product: Product = { id: '1', code: 'SKU-001', description: 'Teclado', balance: 12 };
    const store = {
      products: signal<readonly Product[]>([product]),
      total: signal(total),
      loading: signal(false),
      loaded: signal(true),
      error: signal<ApiError | null>(null),
      load: vi.fn(),
    };
    TestBed.configureTestingModule({
      imports: [ProductListComponent],
      providers: [provideRouter([])],
    });
    TestBed.overrideComponent(ProductListComponent, {
      set: { providers: [{ provide: ProductStore, useValue: store }] },
    });
    return { fixture: TestBed.createComponent(ProductListComponent), store };
  }
});
