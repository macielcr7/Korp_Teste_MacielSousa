import assert from "node:assert/strict";
import { randomUUID } from "node:crypto";

const baseURL = process.env.BASE_URL ?? "http://localhost:4200";

async function request(path, options = {}) {
  const response = await fetch(`${baseURL}${path}`, {
    ...options,
    headers: {
      ...(options.body ? { "content-type": "application/json" } : {}),
      ...options.headers,
    },
  });
  const text = await response.text();
  const body = text ? JSON.parse(text) : undefined;
  if (!response.ok) {
    throw new Error(
      `${options.method ?? "GET"} ${path} returned ${response.status}: ${text}`,
    );
  }
  return { body, headers: response.headers, status: response.status };
}

async function createProduct(balance, suffix) {
  return (
    await request("/api/inventory/v1/products", {
      method: "POST",
      body: JSON.stringify({
        code: `E2E-${Date.now()}-${suffix}`,
        description: `Produto E2E ${suffix}`,
        balance,
      }),
    })
  ).body;
}

async function createInvoice(productId, quantity) {
  return createInvoiceWithItems([{ productId, quantity }]);
}

async function createInvoiceWithItems(items, idempotencyKey = randomUUID()) {
  return (
    await request("/api/billing/v1/invoices", {
      method: "POST",
      headers: { "Idempotency-Key": idempotencyKey },
      body: JSON.stringify({ items }),
    })
  ).body;
}

async function closeInvoice(invoiceId, idempotencyKey) {
  return request(`/api/billing/v1/invoices/${invoiceId}/close`, {
    method: "POST",
    headers: { "Idempotency-Key": idempotencyKey },
  });
}

async function waitForOperation(operationId, timeoutMs = 120_000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const operation = (
      await request(`/api/billing/v1/closure-operations/${operationId}`)
    ).body;
    if (operation.status === "COMPLETED" || operation.status === "FAILED") {
      return operation;
    }
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  throw new Error(
    `operation ${operationId} did not finish within ${timeoutMs}ms`,
  );
}

async function happyPath() {
  const product = await createProduct(10, "HAPPY");
  const createKey = randomUUID();
  const invoice = await createInvoiceWithItems(
    [{ productId: product.id, quantity: 2 }],
    createKey,
  );
  const createReplay = await createInvoiceWithItems(
    [{ productId: product.id, quantity: 2 }],
    createKey,
  );
  assert.equal(invoice.status, "OPEN");
  assert.equal(createReplay.id, invoice.id);
  assert.equal(invoice.items[0].code, product.code);
  assert.equal(invoice.items[0].description, product.description);

  const key = randomUUID();
  const accepted = await closeInvoice(invoice.id, key);
  assert.equal(accepted.status, 202);

  const operation = await waitForOperation(accepted.body.operationId);
  assert.equal(operation.status, "COMPLETED");

  const replay = await closeInvoice(invoice.id, key);
  assert.equal(replay.body.operationId, accepted.body.operationId);
  assert.equal(replay.headers.get("idempotent-replayed"), "true");

  const closedInvoice = (
    await request(`/api/billing/v1/invoices/${invoice.id}`)
  ).body;
  const printable = (
    await request(`/api/billing/v1/invoices/${invoice.id}/printable`)
  ).body;
  const productAfter = (
    await request(`/api/inventory/v1/products/${product.id}`)
  ).body;

  assert.equal(closedInvoice.status, "CLOSED");
  assert.equal(printable.status, "CLOSED");
  assert.equal(productAfter.balance, 8);

  return {
    invoiceId: invoice.id,
    operationId: operation.id,
    initialBalance: 10,
    finalBalance: productAfter.balance,
  };
}

async function atomicFailurePath() {
  const available = await createProduct(5, "ATOMIC-AVAILABLE");
  const unavailable = await createProduct(0, "ATOMIC-UNAVAILABLE");
  const invoice = await createInvoiceWithItems([
    { productId: available.id, quantity: 2 },
    { productId: unavailable.id, quantity: 1 },
  ]);

  const accepted = await closeInvoice(invoice.id, randomUUID());
  const operation = await waitForOperation(accepted.body.operationId);
  assert.equal(operation.status, "FAILED");

  const [availableAfter, unavailableAfter, invoiceAfter] = await Promise.all([
    request(`/api/inventory/v1/products/${available.id}`).then(
      ({ body }) => body,
    ),
    request(`/api/inventory/v1/products/${unavailable.id}`).then(
      ({ body }) => body,
    ),
    request(`/api/billing/v1/invoices/${invoice.id}`).then(({ body }) => body),
  ]);
  assert.equal(availableAfter.balance, 5);
  assert.equal(unavailableAfter.balance, 0);
  assert.equal(invoiceAfter.status, "OPEN");

  return {
    operationStatus: operation.status,
    invoiceStatus: invoiceAfter.status,
    balances: [availableAfter.balance, unavailableAfter.balance],
  };
}

async function concurrencyPath() {
  const product = await createProduct(1, "CONCURRENCY");
  const invoices = await Promise.all([
    createInvoice(product.id, 1),
    createInvoice(product.id, 1),
  ]);
  const accepted = await Promise.all([
    closeInvoice(invoices[0].id, randomUUID()),
    closeInvoice(invoices[1].id, randomUUID()),
  ]);
  const operations = await Promise.all(
    accepted.map(({ body }) => waitForOperation(body.operationId)),
  );
  const completed = operations.filter(({ status }) => status === "COMPLETED");
  const failed = operations.filter(({ status }) => status === "FAILED");
  assert.equal(completed.length, 1);
  assert.equal(failed.length, 1);

  const productAfter = (
    await request(`/api/inventory/v1/products/${product.id}`)
  ).body;
  const invoicesAfter = await Promise.all(
    invoices.map(
      async ({ id }) => (await request(`/api/billing/v1/invoices/${id}`)).body,
    ),
  );
  assert.equal(productAfter.balance, 0);
  assert.deepEqual(
    invoicesAfter.map(({ status }) => status).sort(compareText),
    ["CLOSED", "OPEN"],
  );

  return {
    productId: product.id,
    operationStatuses: operations.map(({ status }) => status).sort(compareText),
    invoiceStatuses: invoicesAfter
      .map(({ status }) => status)
      .sort(compareText),
    finalBalance: productAfter.balance,
  };
}

function compareText(first, second) {
  return first.localeCompare(second);
}

const result = {
  baseURL,
  happyPath: await happyPath(),
  atomicFailurePath: await atomicFailurePath(),
  concurrencyPath: await concurrencyPath(),
};

console.log(JSON.stringify(result, null, 2));
