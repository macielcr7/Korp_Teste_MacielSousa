import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { of, throwError } from 'rxjs';

import { ApiError } from '../../core/errors/api-error';
import { BillingGateway } from '../invoices/application/billing.gateway';
import { ProductGateway } from '../products/application/product.gateway';
import { DashboardComponent } from './dashboard.component';

describe('DashboardComponent', () => {
  it('uses remote totals and dedicated remote result sets for the dashboard', () => {
    const emptyProduct = {
      id: 'empty-1',
      code: 'PRD-001',
      description: 'Sem estoque',
      balance: 0,
    };
    const lowProduct = {
      id: 'low-1',
      code: 'PRD-002',
      description: 'Estoque baixo',
      balance: 2,
    };
    const recentInvoice = {
      id: 'invoice-1',
      number: 42,
      status: 'OPEN' as const,
      createdAt: '2026-08-13T10:00:00Z',
      items: [],
    };
    const productApi = {
      list: vi.fn((query: { status?: string }) => {
        if (query.status === 'low') {
          return of({ items: [lowProduct], total: 17, limit: 5, offset: 0 });
        }
        if (query.status === 'empty') {
          return of({ items: [emptyProduct], total: 4, limit: 5, offset: 0 });
        }
        return of({ items: [], total: 250, limit: 1, offset: 0 });
      }),
    };
    const billingApi = {
      list: vi.fn((query: { status?: string }) => {
        if (query.status === 'OPEN') {
          return of({ items: [], total: 33, limit: 1, offset: 0 });
        }
        if (query.status === 'CLOSED') {
          return of({ items: [], total: 88, limit: 1, offset: 0 });
        }
        return of({ items: [recentInvoice], total: 121, limit: 5, offset: 0 });
      }),
    };
    const fixture = createFixture(productApi, billingApi);

    fixture.detectChanges();

    const root = fixture.nativeElement as HTMLElement;
    const totals = Array.from(root.querySelectorAll<HTMLElement>('.stat-card strong')).map(
      (element) => element.textContent?.trim(),
    );
    expect(totals).toEqual(['250', '17', '33', '88']);
    expect(root.textContent).toContain('Sem estoque');
    expect(root.textContent).toContain('Estoque baixo');
    expect(root.textContent).toContain('NF-42');
    expect(productApi.list).toHaveBeenCalledWith({ limit: 1, offset: 0 });
    expect(productApi.list).toHaveBeenCalledWith({ status: 'low', limit: 5, offset: 0 });
    expect(productApi.list).toHaveBeenCalledWith({ status: 'empty', limit: 5, offset: 0 });
    expect(billingApi.list).toHaveBeenCalledWith({ status: 'OPEN', limit: 1, offset: 0 });
    expect(billingApi.list).toHaveBeenCalledWith({ status: 'CLOSED', limit: 1, offset: 0 });
    expect(billingApi.list).toHaveBeenCalledWith({ limit: 5, offset: 0 });
  });

  it('does not render zeroed indicators as real data after a remote failure', () => {
    const error = new ApiError('Serviço indisponível.', 503, undefined, undefined, true);
    const productApi = { list: vi.fn(() => throwError(() => error)) };
    const billingApi = {
      list: vi.fn(() => of({ items: [], total: 0, limit: 1, offset: 0 })),
    };
    const fixture = createFixture(productApi, billingApi);

    fixture.detectChanges();

    const root = fixture.nativeElement as HTMLElement;
    expect(root.querySelector('.stats-grid')).toBeNull();
    expect(root.textContent).toContain('Serviço indisponível.');
  });

  function createFixture(productApi: object, billingApi: object) {
    TestBed.configureTestingModule({
      imports: [DashboardComponent],
      providers: [
        provideRouter([]),
        { provide: ProductGateway, useValue: productApi },
        { provide: BillingGateway, useValue: billingApi },
      ],
    });
    return TestBed.createComponent(DashboardComponent);
  }
});
