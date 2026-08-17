import { Observable } from 'rxjs';

import {
  CreateProductRequest,
  Product,
  ProductCollection,
  ProductListQuery,
} from '../domain/product';

/** Application port for product catalog operations. */
export abstract class ProductGateway {
  abstract list(query?: ProductListQuery): Observable<ProductCollection>;
  abstract getById(id: string): Observable<Product>;
  abstract create(request: CreateProductRequest): Observable<Product>;
}
