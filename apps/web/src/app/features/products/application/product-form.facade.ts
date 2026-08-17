import { inject, Injectable, signal } from '@angular/core';
import { catchError, EMPTY, finalize, Observable, tap } from 'rxjs';

import { ApiError } from '../../../core/errors/api-error';
import { CreateProductRequest, Product } from '../domain/product';
import { ProductGateway } from './product.gateway';

@Injectable()
export class ProductFormFacade {
  private readonly products = inject(ProductGateway);

  readonly submitting = signal(false);
  readonly error = signal<ApiError | null>(null);

  create(request: CreateProductRequest): Observable<Product> {
    this.submitting.set(true);
    this.error.set(null);
    return this.products.create(request).pipe(
      tap(() => this.error.set(null)),
      catchError((error: ApiError) => {
        this.error.set(error);
        return EMPTY;
      }),
      finalize(() => this.submitting.set(false)),
    );
  }
}
