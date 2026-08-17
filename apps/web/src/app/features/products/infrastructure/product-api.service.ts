import { HttpClient, HttpParams } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';

import { ProductGateway } from '../application/product.gateway';
import {
  CreateProductRequest,
  Product,
  ProductCollection,
  ProductListQuery,
} from '../domain/product';

@Injectable({ providedIn: 'root' })
export class ProductApiService implements ProductGateway {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = '/api/inventory/v1/products';

  list(query: ProductListQuery = {}): Observable<ProductCollection> {
    let params = new HttpParams();
    if (query.search?.trim()) params = params.set('search', query.search.trim());
    if (query.status && query.status !== 'all') params = params.set('status', query.status);
    if (query.limit !== undefined) params = params.set('limit', query.limit);
    if (query.offset !== undefined) params = params.set('offset', query.offset);
    return this.http.get<ProductCollection>(this.baseUrl, { params });
  }

  getById(id: string): Observable<Product> {
    return this.http.get<Product>(`${this.baseUrl}/${encodeURIComponent(id)}`);
  }

  create(request: CreateProductRequest): Observable<Product> {
    return this.http.post<Product>(this.baseUrl, request);
  }
}
