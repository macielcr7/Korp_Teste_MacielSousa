import { HttpErrorResponse, HttpInterceptorFn } from '@angular/common/http';
import { retry, throwError, timer } from 'rxjs';

const transientStatuses = new Set([0, 502, 503, 504]);

export const apiTransientRetryInterceptor: HttpInterceptorFn = (request, next) => {
  const method = request.method.toUpperCase();
  const canRetry =
    method === 'GET' ||
    method === 'HEAD' ||
    method === 'OPTIONS' ||
    request.headers.has('Idempotency-Key');

  if (!canRetry) return next(request);

  return next(request).pipe(
    retry({
      count: 2,
      delay: (error: unknown, retryCount) => {
        if (error instanceof HttpErrorResponse && transientStatuses.has(error.status)) {
          return timer(retryCount * 350);
        }
        return throwError(() => error);
      },
    }),
  );
};
