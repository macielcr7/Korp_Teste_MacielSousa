import { ChangeDetectionStrategy, Component, input } from '@angular/core';

export type AppIconName =
  | 'home'
  | 'products'
  | 'invoices'
  | 'plus'
  | 'search'
  | 'filter'
  | 'arrow'
  | 'printer'
  | 'trash'
  | 'check'
  | 'warning'
  | 'refresh'
  | 'back'
  | 'eye'
  | 'edit';

@Component({
  selector: 'app-icon',
  template: `
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="1.8"
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden="true"
    >
      @switch (name()) {
        @case ('home') {
          <path d="m3 11 9-8 9 8" />
          <path d="M5 10v10h14V10" />
          <path d="M9 20v-6h6v6" />
        }
        @case ('products') {
          <path d="m12 2 8 4.5v11L12 22l-8-4.5v-11Z" />
          <path d="m4.5 6.5 7.5 4 7.5-4M12 10.5V22" />
        }
        @case ('invoices') {
          <path d="M6 2h9l4 4v16H6z" />
          <path d="M14 2v5h5M9 12h6M9 16h6" />
        }
        @case ('plus') {
          <path d="M12 5v14M5 12h14" />
        }
        @case ('search') {
          <circle cx="11" cy="11" r="7" />
          <path d="m20 20-4-4" />
        }
        @case ('filter') {
          <path d="M3 5h18l-7 8v6l-4 2v-8Z" />
        }
        @case ('arrow') {
          <path d="M5 12h14M14 7l5 5-5 5" />
        }
        @case ('printer') {
          <path
            d="M6 9V3h12v6M6 18H4a2 2 0 0 1-2-2v-5a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2v5a2 2 0 0 1-2 2h-2"
          />
          <path d="M6 14h12v8H6z" />
        }
        @case ('trash') {
          <path d="M4 7h16M9 7V4h6v3M7 7l1 14h8l1-14M10 11v6M14 11v6" />
        }
        @case ('check') {
          <path d="m5 12 4 4L19 6" />
        }
        @case ('warning') {
          <path d="M12 3 2.5 20h19Z" />
          <path d="M12 9v4M12 17h.01" />
        }
        @case ('refresh') {
          <path d="M20 7v5h-5M4 17v-5h5" />
          <path d="M6.1 8A7 7 0 0 1 18 6l2 6M17.9 16A7 7 0 0 1 6 18l-2-6" />
        }
        @case ('back') {
          <path d="m15 18-6-6 6-6" />
        }
        @case ('eye') {
          <path d="M2 12s3.5-6 10-6 10 6 10 6-3.5 6-10 6S2 12 2 12Z" />
          <circle cx="12" cy="12" r="2.5" />
        }
        @case ('edit') {
          <path d="M12 20h9" />
          <path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L8 18l-4 1 1-4Z" />
        }
      }
    </svg>
  `,
  styles: `
    :host {
      display: inline-flex;
      width: 1.125rem;
      height: 1.125rem;
      flex: 0 0 auto;
    }
    svg {
      width: 100%;
      height: 100%;
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class AppIconComponent {
  readonly name = input.required<AppIconName>();
}
