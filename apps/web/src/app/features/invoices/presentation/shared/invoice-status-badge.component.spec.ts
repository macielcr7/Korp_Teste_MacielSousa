import { TestBed } from '@angular/core/testing';

import { InvoiceStatusBadgeComponent } from './invoice-status-badge.component';

describe('InvoiceStatusBadgeComponent', () => {
  it.each([
    ['OPEN', false, 'Aberta', false, false],
    ['CLOSED', false, 'Fechada', true, false],
    ['OPEN', true, 'Processando', false, true],
  ] as const)('renders %s with processing=%s', (status, processing, label, closed, active) => {
    TestBed.configureTestingModule({ imports: [InvoiceStatusBadgeComponent] });
    const fixture = TestBed.createComponent(InvoiceStatusBadgeComponent);
    fixture.componentRef.setInput('status', status);
    fixture.componentRef.setInput('processing', processing);
    fixture.detectChanges();

    expect(fixture.nativeElement.textContent.trim()).toBe(label);
    expect(fixture.nativeElement.classList.contains('closed')).toBe(closed);
    expect(fixture.nativeElement.classList.contains('processing')).toBe(active);
  });
});
