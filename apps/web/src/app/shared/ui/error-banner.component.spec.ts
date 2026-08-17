import { TestBed } from '@angular/core/testing';

import { ApiError } from '../../core/errors/api-error';
import { ErrorBannerComponent } from './error-banner.component';

describe('ErrorBannerComponent', () => {
  it('does not offer a retry for a terminal error by default', () => {
    const fixture = TestBed.createComponent(ErrorBannerComponent);
    fixture.componentRef.setInput(
      'error',
      new ApiError('Dados inválidos.', 422, undefined, undefined, false),
    );
    fixture.detectChanges();

    expect(fixture.nativeElement.querySelector('button')).toBeNull();
  });

  it('offers a retry for a retryable error by default', () => {
    const fixture = TestBed.createComponent(ErrorBannerComponent);
    fixture.componentRef.setInput(
      'error',
      new ApiError('Serviço indisponível.', 503, undefined, undefined, true),
    );
    fixture.detectChanges();

    expect(fixture.nativeElement.querySelector('button')?.textContent).toContain(
      'Tentar novamente',
    );
  });

  it('shows the contextual title and a clear trace label', () => {
    const fixture = TestBed.createComponent(ErrorBannerComponent);
    fixture.componentRef.setInput('heading', 'Não foi possível emitir a nota');
    fixture.componentRef.setInput(
      'error',
      new ApiError('Saldo insuficiente.', 409, 'INSUFFICIENT_STOCK', 'trace-123'),
    );
    fixture.detectChanges();

    expect(fixture.nativeElement.textContent).toContain('Não foi possível emitir a nota');
    expect(fixture.nativeElement.textContent).toContain('Código de rastreio:');
  });
});
