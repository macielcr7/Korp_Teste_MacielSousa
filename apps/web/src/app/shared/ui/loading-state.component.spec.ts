import { TestBed } from '@angular/core/testing';

import { LoadingStateComponent } from './loading-state.component';

describe('LoadingStateComponent', () => {
  it('renders an accessible loading message', () => {
    TestBed.configureTestingModule({ imports: [LoadingStateComponent] });
    const fixture = TestBed.createComponent(LoadingStateComponent);
    fixture.componentRef.setInput('message', 'Carregando dados…');
    fixture.detectChanges();

    expect(fixture.nativeElement.getAttribute('aria-live')).toBe('polite');
    expect(fixture.nativeElement.getAttribute('aria-busy')).toBe('true');
    expect(fixture.nativeElement.textContent).toContain('Carregando dados…');
  });
});
