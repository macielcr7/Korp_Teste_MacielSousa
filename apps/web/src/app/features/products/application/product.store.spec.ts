import { TestBed } from '@angular/core/testing';
import { of, throwError } from 'rxjs';

import { ApiError } from '../../../core/errors/api-error';
import { ProductGateway } from './product.gateway';
import { ProductStore } from './product.store';

describe('ProductStore', () => {
  it('clears results from the previous query when a remote filter fails', () => {
    const product = { id: 'product-1', code: 'PRD-001', description: 'Produto', balance: 10 };
    const error = new ApiError('Serviço indisponível.', 503, undefined, undefined, true);
    const api = {
      list: vi
        .fn()
        .mockReturnValueOnce(of({ items: [product], total: 1, limit: 10, offset: 0 }))
        .mockReturnValueOnce(throwError(() => error)),
    };
    TestBed.configureTestingModule({
      providers: [ProductStore, { provide: ProductGateway, useValue: api }],
    });
    const store = TestBed.inject(ProductStore);

    store.load({ limit: 10, offset: 0 });
    expect(store.products()).toEqual([product]);

    store.load({ status: 'empty', limit: 10, offset: 0 });

    expect(store.products()).toEqual([]);
    expect(store.total()).toBe(0);
    expect(store.error()).toBe(error);
  });
});
