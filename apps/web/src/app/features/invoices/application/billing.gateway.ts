import { Observable } from 'rxjs';

import {
  ClosureOperation,
  CreateInvoiceRequest,
  Invoice,
  InvoiceCollection,
  InvoiceListQuery,
} from '../domain/invoice';

/** Application port for billing operations. */
export abstract class BillingGateway {
  abstract list(query?: InvoiceListQuery): Observable<InvoiceCollection>;
  abstract getById(id: string): Observable<Invoice>;
  abstract create(request: CreateInvoiceRequest, idempotencyKey: string): Observable<Invoice>;
  abstract close(id: string, idempotencyKey: string): Observable<ClosureOperation>;
  abstract getClosureOperation(operationId: string): Observable<ClosureOperation>;
  abstract getPrintable(id: string): Observable<Invoice>;
}
