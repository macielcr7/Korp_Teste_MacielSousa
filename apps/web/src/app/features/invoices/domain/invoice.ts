export type InvoiceStatus = 'OPEN' | 'CLOSED';
export type ClosureOperationStatus = 'PENDING' | 'PROCESSING' | 'RETRYING' | 'COMPLETED' | 'FAILED';

export interface InvoiceItem {
  readonly productId: string;
  readonly productCode: string;
  readonly productDescription: string;
  readonly quantity: number;
}

export interface ClosureOperationSummary {
  readonly operationId: string;
  readonly status: ClosureOperationStatus;
}

export interface Invoice {
  readonly id: string;
  readonly number: number;
  readonly status: InvoiceStatus;
  readonly items: readonly InvoiceItem[];
  readonly createdAt: string;
  readonly closedAt?: string | null;
  readonly activeClosureOperation?: ClosureOperationSummary | null;
}

export interface CreateInvoiceRequest {
  readonly items: readonly {
    readonly productId: string;
    readonly quantity: number;
  }[];
}

export type InvoiceListFilter = 'all' | InvoiceStatus;

export interface InvoiceListQuery {
  readonly status?: InvoiceStatus;
  readonly limit?: number;
  readonly offset?: number;
}

export interface InvoiceCollection {
  readonly items: readonly Invoice[];
  readonly total: number;
  readonly limit: number;
  readonly offset: number;
}

export interface ClosureOperation extends ClosureOperationSummary {
  readonly invoiceId?: string;
  readonly attempts?: number;
  readonly nextAttemptAt?: string | null;
  readonly lastError?: string;
  readonly retryable?: boolean;
}

export function isTerminalOperation(status: ClosureOperationStatus): boolean {
  return status === 'COMPLETED' || status === 'FAILED';
}
