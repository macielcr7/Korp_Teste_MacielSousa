export interface Product {
  readonly id: string;
  readonly code: string;
  readonly description: string;
  readonly balance: number;
  readonly createdAt?: string;
  readonly updatedAt?: string;
}

export interface CreateProductRequest {
  readonly code: string;
  readonly description: string;
  readonly balance: number;
}

export type ProductStockFilter = 'all' | 'active' | 'low' | 'empty';

export interface ProductListQuery {
  readonly search?: string;
  readonly status?: ProductStockFilter;
  readonly limit?: number;
  readonly offset?: number;
}

export interface ProductCollection {
  readonly items: readonly Product[];
  readonly total: number;
  readonly limit: number;
  readonly offset: number;
}
