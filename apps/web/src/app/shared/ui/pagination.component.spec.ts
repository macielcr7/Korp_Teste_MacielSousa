import { ComponentFixture, TestBed } from '@angular/core/testing';

import { PaginationComponent } from './pagination.component';

describe('PaginationComponent', () => {
  let fixture: ComponentFixture<PaginationComponent>;

  beforeEach(() => {
    TestBed.configureTestingModule({ imports: [PaginationComponent] });
    fixture = TestBed.createComponent(PaginationComponent);
  });

  it('does not render when the collection is empty', () => {
    render({ total: 0, currentPage: 1, pageSize: 10, itemLabel: 'itens' });

    expect(fixture.nativeElement.querySelector('footer')).toBeNull();
  });

  it('shows the visible interval and a window of at most four pages', () => {
    render({ total: 95, currentPage: 5, pageSize: 10, itemLabel: 'produtos' });

    expect(fixture.nativeElement.textContent).toContain('Exibindo 41-50 de 95 produtos');
    expect(numberedButtons().map((button) => button.textContent?.trim())).toEqual([
      '4',
      '5',
      '6',
      '7',
    ]);
  });

  it('emits only valid page changes', () => {
    render({ total: 15, currentPage: 1, pageSize: 10, itemLabel: 'notas' });
    const changes: number[] = [];
    fixture.componentInstance.pageChange.subscribe((page) => changes.push(page));

    previousButton().click();
    nextButton().click();

    expect(changes).toEqual([2]);
  });

  it('disables navigation after the last page', () => {
    render({ total: 15, currentPage: 2, pageSize: 10, itemLabel: 'notas' });

    expect(previousButton().disabled).toBe(false);
    expect(nextButton().disabled).toBe(true);
  });

  function render(inputs: {
    total: number;
    currentPage: number;
    pageSize: number;
    itemLabel: string;
  }): void {
    for (const [name, value] of Object.entries(inputs)) fixture.componentRef.setInput(name, value);
    fixture.detectChanges();
  }

  function numberedButtons(): HTMLButtonElement[] {
    return Array.from(fixture.nativeElement.querySelectorAll('button:not([aria-label])'));
  }

  function previousButton(): HTMLButtonElement {
    return fixture.nativeElement.querySelector('button[aria-label="Página anterior"]');
  }

  function nextButton(): HTMLButtonElement {
    return fixture.nativeElement.querySelector('button[aria-label="Próxima página"]');
  }
});
