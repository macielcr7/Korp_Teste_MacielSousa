import { HttpClient, HttpHeaders, HttpParams } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { map, Observable } from 'rxjs';

import { BillingGateway } from '../application/billing.gateway';
import {
  ClosureOperation,
  CreateInvoiceRequest,
  Invoice,
  InvoiceCollection,
  InvoiceListQuery,
  InvoiceStatus,
} from '../domain/invoice';

interface InvoiceItemDto {
  readonly productId: string;
  readonly code: string;
  readonly description: string;
  readonly quantity: number;
}

interface InvoiceDto {
  readonly id: string;
  readonly number: number;
  readonly status: InvoiceStatus;
  readonly items: readonly InvoiceItemDto[];
  readonly createdAt: string;
  readonly closedAt?: string | null;
  readonly activeClosureOperation?: Invoice['activeClosureOperation'];
}

interface InvoiceCollectionDto {
  readonly items: readonly InvoiceDto[];
  readonly total: number;
  readonly limit: number;
  readonly offset: number;
}

type ClosureOperationDto = Omit<ClosureOperation, 'operationId'> & {
  readonly id: string;
};

@Injectable({ providedIn: 'root' })
export class BillingApiService implements BillingGateway {
  private readonly http = inject(HttpClient);
  private readonly invoiceUrl = '/api/billing/v1/invoices';
  private readonly operationUrl = '/api/billing/v1/closure-operations';

  list(query: InvoiceListQuery = {}): Observable<InvoiceCollection> {
    let params = new HttpParams();
    if (query.status) params = params.set('status', query.status);
    if (query.limit !== undefined) params = params.set('limit', query.limit);
    if (query.offset !== undefined) params = params.set('offset', query.offset);
    return this.http.get<InvoiceCollectionDto>(this.invoiceUrl, { params }).pipe(
      map((response) => ({
        items: response.items.map(mapInvoice),
        total: response.total,
        limit: response.limit,
        offset: response.offset,
      })),
    );
  }

  getById(id: string): Observable<Invoice> {
    return this.http
      .get<InvoiceDto>(`${this.invoiceUrl}/${encodeURIComponent(id)}`)
      .pipe(map(mapInvoice));
  }

  create(request: CreateInvoiceRequest, idempotencyKey: string): Observable<Invoice> {
    const headers = new HttpHeaders({ 'Idempotency-Key': idempotencyKey });
    return this.http.post<InvoiceDto>(this.invoiceUrl, request, { headers }).pipe(map(mapInvoice));
  }

  close(id: string, idempotencyKey: string): Observable<ClosureOperation> {
    const headers = new HttpHeaders({ 'Idempotency-Key': idempotencyKey });
    return this.http.post<ClosureOperation>(
      `${this.invoiceUrl}/${encodeURIComponent(id)}/close`,
      {},
      { headers },
    );
  }

  getClosureOperation(operationId: string): Observable<ClosureOperation> {
    return this.http
      .get<ClosureOperationDto>(`${this.operationUrl}/${encodeURIComponent(operationId)}`)
      .pipe(map(({ id, ...operation }) => ({ ...operation, operationId: id })));
  }

  getPrintable(id: string): Observable<Invoice> {
    return this.http
      .get<InvoiceDto>(`${this.invoiceUrl}/${encodeURIComponent(id)}/printable`)
      .pipe(map(mapInvoice));
  }
}

function mapInvoice(dto: InvoiceDto): Invoice {
  return {
    id: dto.id,
    number: dto.number,
    status: dto.status,
    createdAt: dto.createdAt,
    closedAt: dto.closedAt,
    activeClosureOperation: dto.activeClosureOperation,
    items: dto.items.map((item) => ({
      productId: item.productId,
      productCode: item.code,
      productDescription: item.description,
      quantity: item.quantity,
    })),
  };
}
