import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { ProductCollection } from '../domain/product';
import { ProductApiService } from './product-api.service';

describe('ProductApiService', () => {
  let service: ProductApiService;
  let http: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    service = TestBed.inject(ProductApiService);
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => http.verify());

  it('sends remote filters and returns the paginated collection', () => {
    const product = { id: '1', code: 'ABC', description: 'Produto', balance: 12 };
    const collection = { items: [product], total: 21, limit: 10, offset: 10 };
    let result: ProductCollection | undefined;

    service
      .list({ search: ' abc ', status: 'low', limit: 10, offset: 10 })
      .subscribe((response) => (result = response));
    const request = http.expectOne(
      (candidate) =>
        candidate.url === '/api/inventory/v1/products' &&
        candidate.params.get('search') === 'abc' &&
        candidate.params.get('status') === 'low' &&
        candidate.params.get('limit') === '10' &&
        candidate.params.get('offset') === '10',
    );
    request.flush(collection);

    expect(result).toEqual(collection);
  });

  it('omits the all status from the remote query', () => {
    service.list({ status: 'all', limit: 10, offset: 0 }).subscribe();

    const request = http.expectOne('/api/inventory/v1/products?limit=10&offset=0');
    expect(request.request.params.has('status')).toBe(false);
    request.flush({ items: [], total: 0, limit: 10, offset: 0 });
  });

  it('sends the product payload in camelCase', () => {
    const requestBody = { code: 'ABC', description: 'Produto', balance: 12 };
    service.create(requestBody).subscribe();

    const request = http.expectOne('/api/inventory/v1/products');
    expect(request.request.method).toBe('POST');
    expect(request.request.body).toEqual(requestBody);
    request.flush({ id: '1', ...requestBody });
  });
});
