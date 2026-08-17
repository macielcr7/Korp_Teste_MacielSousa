import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';

import { ApiError } from '../../core/errors/api-error';
import { AppIconComponent } from './app-icon.component';

@Component({
  selector: 'app-error-banner',
  imports: [AppIconComponent, MatButtonModule],
  template: `
    @if (error(); as currentError) {
      <section class="error" role="alert">
        <span class="error-icon"><app-icon name="warning" /></span>
        <div>
          <strong>{{ errorTitle(currentError) }}</strong>
          <p>{{ currentError.message }}</p>
          @if (currentError.traceId) {
            <p class="trace">
              Código de rastreio: <code>{{ currentError.traceId }}</code>
            </p>
          }
        </div>
        @if (canRetry(currentError)) {
          <button mat-stroked-button type="button" (click)="retry.emit()">
            <app-icon name="refresh" /> Tentar novamente
          </button>
        }
      </section>
    }
  `,
  styles: `
    :host {
      display: block;
    }
    .error {
      display: grid;
      grid-template-columns: auto 1fr auto;
      align-items: center;
      gap: 0.8rem;
      margin-bottom: 1rem;
      padding: 0.85rem 1rem;
      border: 1px solid #f5aaaa;
      border-radius: var(--kf-radius);
      color: #991b1b;
      background: #fff1f2;
    }
    .error-icon {
      display: grid;
      width: 2rem;
      height: 2rem;
      place-items: center;
      border-radius: 50%;
      background: var(--kf-error-bg);
    }
    strong {
      font-size: 0.8rem;
      font-weight: 700;
    }
    p {
      margin: 0.2rem 0 0;
      font-size: 0.72rem;
      line-height: 1.4;
    }
    .trace {
      font-size: 0.64rem;
      overflow-wrap: anywhere;
    }
    button {
      display: inline-flex;
      align-items: center;
      gap: 0.35rem;
      white-space: nowrap;
    }
    @media (max-width: 34rem) {
      .error {
        grid-template-columns: auto 1fr;
      }
      button {
        grid-column: 1/-1;
        width: 100%;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ErrorBannerComponent {
  readonly error = input<ApiError | null>(null);
  readonly heading = input<string>();
  readonly retryable = input<boolean | undefined>(undefined);
  readonly retry = output<void>();

  protected canRetry(error: ApiError): boolean {
    return this.retryable() ?? error.retryable;
  }

  protected errorTitle(error: ApiError): string {
    return this.heading()?.trim() || error.title?.trim() || 'Falha ao processar a solicitação';
  }
}
