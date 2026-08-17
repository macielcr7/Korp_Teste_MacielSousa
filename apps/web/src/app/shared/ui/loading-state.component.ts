import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';

@Component({
  selector: 'app-loading-state',
  imports: [MatProgressSpinnerModule],
  host: {
    'aria-live': 'polite',
    'aria-busy': 'true',
  },
  template: `<mat-spinner [diameter]="diameter()" /><span>{{ message() }}</span>`,
  styles: `
    :host {
      display: grid;
      min-height: 14rem;
      place-items: center;
      align-content: center;
      gap: 0.75rem;
      padding: 2rem;
      text-align: center;
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class LoadingStateComponent {
  readonly message = input.required<string>();
  readonly diameter = input(36);
}
