import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { ApiError } from '../../../../core/errors/api-error';
import { InvoiceStore } from '../../application/invoice.store';
import { Invoice } from '../../domain/invoice';
import { InvoiceListComponent } from './invoice-list.component';

describe('InvoiceListComponent', () => {
  it('requests ten invoices per page and filters remotely', () => {
    const { fixture, store } = createFixture(21);
    fixture.detectChanges();

    expect(store.load).toHaveBeenCalledWith({ status: undefined, limit: 10, offset: 0 });

    const root = fixture.nativeElement as HTMLElement;
    const filterButtons = Array.from(
      root.querySelectorAll<HTMLButtonElement>('.filter-tabs button'),
    );
    expect(filterButtons.map((button) => button.textContent?.trim())).toEqual([
      'Todas',
      'Abertas',
      'Fechadas',
    ]);
    filterButtons[2].click();
    expect(store.load).toHaveBeenLastCalledWith({ status: 'CLOSED', limit: 10, offset: 0 });

    root.querySelector<HTMLButtonElement>('button[aria-label="Próxima página"]')?.click();
    expect(store.load).toHaveBeenLastCalledWith({ status: 'CLOSED', limit: 10, offset: 10 });
    fixture.destroy();
  });

  it('reloads the last valid remote page when the total shrinks', async () => {
    const { fixture, store } = createFixture(25);
    fixture.detectChanges();
    const root = fixture.nativeElement as HTMLElement;

    root.querySelector<HTMLButtonElement>('button[aria-label="Próxima página"]')?.click();
    fixture.detectChanges();
    root.querySelector<HTMLButtonElement>('button[aria-label="Próxima página"]')?.click();
    expect(store.load).toHaveBeenLastCalledWith({ status: undefined, limit: 10, offset: 20 });

    store.total.set(15);
    fixture.detectChanges();
    await fixture.whenStable();

    expect(store.load).toHaveBeenLastCalledWith({ status: undefined, limit: 10, offset: 10 });
    fixture.destroy();
  });

  function createFixture(total: number) {
    const invoice: Invoice = {
      id: 'invoice-1',
      number: 42,
      status: 'CLOSED',
      createdAt: '2026-08-13T10:00:00Z',
      closedAt: '2026-08-13T10:01:00Z',
      items: [
        {
          productId: 'product-1',
          productCode: 'SKU-001',
          productDescription: 'Produto',
          quantity: 2,
        },
      ],
    };
    const store = {
      invoices: signal<readonly Invoice[]>([invoice]),
      total: signal(total),
      loading: signal(false),
      loaded: signal(true),
      error: signal<ApiError | null>(null),
      load: vi.fn(),
    };
    TestBed.configureTestingModule({
      imports: [InvoiceListComponent],
      providers: [provideRouter([])],
    });
    TestBed.overrideComponent(InvoiceListComponent, {
      set: { providers: [{ provide: InvoiceStore, useValue: store }] },
    });
    return { fixture: TestBed.createComponent(InvoiceListComponent), store };
  }
});
