import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, provideRouter } from '@angular/router';
import { Subject } from 'rxjs';

import { BillingGateway } from '../../application/billing.gateway';
import { Invoice } from '../../domain/invoice';
import { InvoicePrintComponent } from './invoice-print.component';

describe('InvoicePrintComponent', () => {
  it('offers printing only after the printable contract returns a closed invoice', () => {
    const response = new Subject<Invoice>();
    const api = { getPrintable: vi.fn(() => response.asObservable()) };
    TestBed.configureTestingModule({
      imports: [InvoicePrintComponent],
      providers: [
        provideRouter([]),
        { provide: BillingGateway, useValue: api },
        {
          provide: ActivatedRoute,
          useValue: { snapshot: { paramMap: { get: () => 'invoice-1' } } },
        },
      ],
    });
    const fixture = TestBed.createComponent(InvoicePrintComponent);
    fixture.detectChanges();

    expect(printButton(fixture.nativeElement)).toBeNull();

    response.next({
      id: 'invoice-1',
      number: 1,
      status: 'CLOSED',
      createdAt: '2026-08-13T10:00:00Z',
      closedAt: '2026-08-13T10:01:00Z',
      items: [],
    });
    response.complete();
    fixture.detectChanges();

    const print = vi.spyOn(window, 'print').mockImplementation(() => undefined);
    printButton(fixture.nativeElement)?.click();
    expect(print).toHaveBeenCalledOnce();
    expect(api.getPrintable).toHaveBeenCalledWith('invoice-1');
  });

  function printButton(element: HTMLElement): HTMLButtonElement | null {
    return (
      Array.from(element.querySelectorAll<HTMLButtonElement>('button')).find((button) =>
        button.textContent?.includes('Imprimir'),
      ) ?? null
    );
  }
});
