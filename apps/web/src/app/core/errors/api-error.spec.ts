import { HttpErrorResponse, HttpHeaders } from '@angular/common/http';

import { toApiError } from './api-error';

describe('toApiError', () => {
  it('maps Problem Details and preserves the trace id', () => {
    const result = toApiError(
      new HttpErrorResponse({
        status: 409,
        headers: new HttpHeaders(),
        error: {
          title: 'Saldo insuficiente',
          detail: 'O produto ABC não possui saldo.',
          code: 'INSUFFICIENT_STOCK',
          traceId: 'trace-123',
          retryable: false,
        },
      }),
    );

    expect(result.message).toBe('O produto ABC não possui saldo.');
    expect(result.title).toBe('Saldo insuficiente');
    expect(result.status).toBe(409);
    expect(result.code).toBe('INSUFFICIENT_STOCK');
    expect(result.traceId).toBe('trace-123');
    expect(result.retryable).toBe(false);
  });

  it('uses a connection-oriented fallback for network errors', () => {
    const result = toApiError(new HttpErrorResponse({ status: 0 }));

    expect(result.retryable).toBe(true);
    expect(result.message).toContain('conectar');
  });

  it('uses the request id response header as a correlation fallback', () => {
    const result = toApiError(
      new HttpErrorResponse({
        status: 500,
        headers: new HttpHeaders({ 'X-Request-ID': 'request-123' }),
        error: { code: 'INTERNAL_ERROR', detail: 'Erro interno.' },
      }),
    );

    expect(result.traceId).toBe('request-123');
  });

  it('translates a legacy English server message', () => {
    const result = toApiError(
      new HttpErrorResponse({
        status: 409,
        error: { detail: 'insufficient product balance' },
      }),
    );

    expect(result.message).toBe('Saldo insuficiente para um ou mais produtos da nota.');
  });

  it('preserves a specific Portuguese validation detail from the server', () => {
    const result = toApiError(
      new HttpErrorResponse({
        status: 422,
        error: {
          code: 'VALIDATION_ERROR',
          detail: 'A nota deve conter no máximo 20 itens.',
        },
      }),
    );

    expect(result.message).toBe('A nota deve conter no máximo 20 itens.');
  });

  it('does not expose an internal server detail', () => {
    const result = toApiError(
      new HttpErrorResponse({
        status: 500,
        error: { code: 'INTERNAL_ERROR', detail: 'failed to query postgres table invoices' },
      }),
    );

    expect(result.message).toContain('código de rastreio');
    expect(result.message).not.toContain('postgres');
  });
});
