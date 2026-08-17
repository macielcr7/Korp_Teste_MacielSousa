import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { App } from './app';

describe('App', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [App],
      providers: [provideRouter([])],
    }).compileComponents();
  });

  it('renders the primary navigation', () => {
    const fixture = TestBed.createComponent(App);
    fixture.detectChanges();

    const links = Array.from(
      (fixture.nativeElement as HTMLElement).querySelectorAll<HTMLAnchorElement>('.desktop-nav a'),
    ).map((link) => link.textContent?.trim());

    expect(links).toEqual(['Produtos', 'Notas']);
  });

  it('keeps mobile navigation useful without rendering inactive toolbar controls', () => {
    const fixture = TestBed.createComponent(App);
    fixture.detectChanges();
    const root = fixture.nativeElement as HTMLElement;

    const mobileLinks = Array.from(root.querySelectorAll<HTMLAnchorElement>('.mobile-nav a')).map(
      (link) => link.textContent?.trim(),
    );

    expect(mobileLinks).toEqual(['Produtos', 'Notas']);
    expect(root.querySelector('.mobile-brand')?.getAttribute('href')).toBe('/');
    expect(root.querySelector('.topbar button')).toBeNull();
  });
});
