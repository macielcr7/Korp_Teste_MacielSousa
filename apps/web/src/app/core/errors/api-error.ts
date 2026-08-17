import { HttpErrorResponse } from '@angular/common/http';

export interface ProblemDetails {
  readonly type?: string;
  readonly title?: string;
  readonly status?: number;
  readonly code?: string;
  readonly detail?: string;
  readonly traceId?: string;
  readonly retryable?: boolean;
  readonly errors?: readonly unknown[];
}

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly code?: string,
    readonly traceId?: string,
    readonly retryable = false,
    readonly title?: string,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

const messagesByCode: Readonly<Record<string, string>> = {
  CLOSURE_ALREADY_REQUESTED: 'A emissão desta nota já foi solicitada.',
  CLOSURE_OPERATION_NOT_FOUND: 'A operação de emissão não foi encontrada.',
  DUPLICATE_PRODUCT_CODE: 'Já existe um produto com este código.',
  IDEMPOTENCY_CONFLICT: 'A solicitação já foi usada com dados diferentes.',
  IDEMPOTENCY_KEY_CONFLICT: 'A chave de emissão já foi usada para outra nota.',
  IDEMPOTENCY_KEY_REQUIRED:
    'A identificação desta solicitação não foi informada. Recarregue a página e tente novamente.',
  INSUFFICIENT_STOCK: 'Saldo insuficiente para um ou mais produtos da nota.',
  INTERNAL_ERROR:
    'O servidor não conseguiu processar a solicitação. Tente novamente e, se o problema persistir, informe o código de rastreio.',
  INVALID_ID: 'O identificador informado é inválido.',
  INVALID_IDEMPOTENCY_KEY: 'A identificação da solicitação de emissão é inválida.',
  INVALID_JSON: 'Os dados enviados são inválidos.',
  INVALID_PRODUCT: 'Revise os dados informados para o produto.',
  INVALID_QUERY: 'Os filtros informados são inválidos.',
  INVALID_REQUEST: 'Os dados enviados são inválidos.',
  INVALID_STOCK_COMMAND: 'A movimentação de estoque é inválida.',
  INVENTORY_UNAVAILABLE:
    'O serviço de estoque está temporariamente indisponível. Aguarde alguns instantes e tente novamente.',
  INVOICE_NOT_CLOSED: 'Somente notas emitidas podem ser impressas.',
  INVOICE_NOT_FOUND: 'A nota não foi encontrada.',
  INVOICE_NOT_OPEN: 'A nota não está mais aberta.',
  NOT_READY:
    'O serviço ainda está iniciando ou perdeu acesso ao banco de dados. Aguarde alguns instantes e tente novamente.',
  PRODUCT_NOT_FOUND: 'Um dos produtos selecionados não foi encontrado.',
  STOCK_COMMAND_NOT_FOUND: 'A movimentação de estoque não foi encontrada.',
  VALIDATION_ERROR: 'Um ou mais dados são inválidos. Revise os campos informados.',
};

const titlesByCode: Readonly<Record<string, string>> = {
  CLOSURE_ALREADY_REQUESTED: 'Emissão já solicitada',
  CLOSURE_OPERATION_NOT_FOUND: 'Emissão não encontrada',
  DUPLICATE_PRODUCT_CODE: 'Código de produto já cadastrado',
  IDEMPOTENCY_CONFLICT: 'Solicitação em conflito',
  IDEMPOTENCY_KEY_CONFLICT: 'Solicitação em conflito',
  INSUFFICIENT_STOCK: 'Saldo insuficiente',
  INTERNAL_ERROR: 'Falha no servidor',
  INVALID_PRODUCT: 'Produto inválido',
  INVENTORY_UNAVAILABLE: 'Estoque indisponível',
  INVOICE_NOT_CLOSED: 'Nota ainda não emitida',
  INVOICE_NOT_FOUND: 'Nota não encontrada',
  INVOICE_NOT_OPEN: 'Nota já processada',
  PRODUCT_NOT_FOUND: 'Produto não encontrado',
  VALIDATION_ERROR: 'Dados inválidos',
};

export function localizeServerMessage(
  message: string | undefined,
  code?: string,
  fallback = 'A solicitação não pôde ser processada. Verifique os dados informados e tente novamente.',
): string {
  if (code === 'INTERNAL_ERROR') return messagesByCode['INTERNAL_ERROR'];
  const normalized = message?.trim().toLocaleLowerCase('en-US') ?? '';
  if (
    normalized.includes('insufficient product balance') ||
    normalized.includes('insufficient stock')
  ) {
    return messagesByCode['INSUFFICIENT_STOCK'];
  }
  if (normalized.includes('invoice no longer exists')) return messagesByCode['INVOICE_NOT_FOUND'];
  if (normalized.includes('only closed invoices can be printed')) {
    return messagesByCode['INVOICE_NOT_CLOSED'];
  }
  if (
    normalized.includes('inventory unavailable') ||
    normalized.includes('inventory service unavailable') ||
    normalized.includes('connection refused') ||
    normalized.includes('context deadline exceeded')
  ) {
    return messagesByCode['INVENTORY_UNAVAILABLE'];
  }
  if (
    normalized.includes('não foi possível concluir a emissão. tente novamente') ||
    normalized.includes('a baixa de estoque foi rejeitada')
  ) {
    return 'A emissão foi rejeitada pelo estoque. Verifique os produtos e as quantidades da nota antes de tentar novamente.';
  }
  return message?.trim() || (code ? messagesByCode[code] : undefined) || fallback;
}

export function toApiError(response: HttpErrorResponse): ApiError {
  const problem = isProblemDetails(response.error) ? response.error : undefined;
  const fallback = fallbackByStatus(response.status);
  const code = problem?.code;
  const message =
    response.status >= 500 && (!code || code === 'INTERNAL_ERROR')
      ? messagesByCode['INTERNAL_ERROR']
      : localizeServerMessage(problem?.detail ?? problem?.title, code, fallback);

  return new ApiError(
    message,
    response.status,
    code,
    problem?.traceId ??
      response.headers.get('x-trace-id') ??
      response.headers.get('x-request-id') ??
      undefined,
    problem?.retryable ?? (response.status === 0 || response.status >= 500),
    (code && titlesByCode[code]) || problem?.title,
  );
}

function fallbackByStatus(status: number): string {
  if (status === 0) {
    return 'A aplicação não conseguiu se conectar ao servidor. Confirme se os serviços estão em execução e tente novamente.';
  }
  if (status === 400 || status === 422) {
    return 'A solicitação contém dados inválidos. Revise os campos informados.';
  }
  if (status === 404) return 'O recurso solicitado não foi encontrado.';
  if (status === 409) {
    return 'A operação entrou em conflito com o estado atual do recurso. Atualize a página e tente novamente.';
  }
  if (status === 503) {
    return 'O serviço necessário está temporariamente indisponível. Aguarde alguns instantes e tente novamente.';
  }
  return 'O servidor não conseguiu processar a solicitação. Tente novamente e use o código de rastreio caso o problema persista.';
}

function isProblemDetails(value: unknown): value is ProblemDetails {
  return typeof value === 'object' && value !== null;
}
