import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { randomUUID } from "node:crypto";
import { existsSync } from "node:fs";
import { isAbsolute } from "node:path";

const baseURL = process.env.BASE_URL ?? "http://localhost:4200";
const dockerExecutable = resolveDockerExecutable();

function resolveDockerExecutable() {
  const configured = process.env.DOCKER_CLI_PATH?.trim();
  if (configured) {
    if (!isAbsolute(configured) || !existsSync(configured)) {
      throw new Error(
        "DOCKER_CLI_PATH deve apontar para um executável absoluto existente.",
      );
    }
    return configured;
  }

  const candidates =
    process.platform === "win32"
      ? [String.raw`C:\Program Files\Docker\Docker\resources\bin\docker.exe`]
      : [
          "/usr/bin/docker",
          "/usr/local/bin/docker",
          "/opt/homebrew/bin/docker",
        ];
  const executable = candidates.find((candidate) => existsSync(candidate));
  if (!executable) {
    throw new Error(
      "Docker CLI não encontrado. Configure DOCKER_CLI_PATH com o caminho absoluto.",
    );
  }
  return executable;
}

async function request(path, options = {}) {
  const response = await fetch(`${baseURL}${path}`, {
    ...options,
    headers: {
      ...(options.body ? { "content-type": "application/json" } : {}),
      ...options.headers,
    },
  });
  const text = await response.text();
  if (!response.ok) {
    throw new Error(
      `${options.method ?? "GET"} ${path} returned ${response.status}: ${text}`,
    );
  }
  return text ? JSON.parse(text) : undefined;
}

async function waitUntil(operationId, predicate, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const operation = await request(
      `/api/billing/v1/closure-operations/${operationId}`,
    );
    if (predicate(operation)) {
      return operation;
    }
    await new Promise((resolve) => setTimeout(resolve, 500));
  }
  throw new Error(`operation ${operationId} did not reach the expected state`);
}

async function waitForInventory(timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      const response = await fetch("http://localhost:8081/ready");
      if (response.ok) {
        return;
      }
    } catch {
      // Expected while the service is starting.
    }
    await new Promise((resolve) => setTimeout(resolve, 500));
  }
  throw new Error("Inventory Service did not become ready");
}

const product = await request("/api/inventory/v1/products", {
  method: "POST",
  body: JSON.stringify({
    code: `E2E-RECOVERY-${Date.now()}`,
    description: "Produto para recuperação de falha",
    balance: 5,
  }),
});
const invoice = await request("/api/billing/v1/invoices", {
  method: "POST",
  headers: { "Idempotency-Key": randomUUID() },
  body: JSON.stringify({ items: [{ productId: product.id, quantity: 3 }] }),
});

let inventoryRestored = false;
try {
  execFileSync(dockerExecutable, ["compose", "stop", "inventory"], {
    stdio: "inherit",
  });

  const accepted = await request(
    `/api/billing/v1/invoices/${invoice.id}/close`,
    {
      method: "POST",
      headers: { "Idempotency-Key": randomUUID() },
    },
  );
  const duringFailure = await waitUntil(
    accepted.operationId,
    ({ status }) => status === "RETRYING",
    120_000,
  );
  const invoiceDuringFailure = await request(
    `/api/billing/v1/invoices/${invoice.id}`,
  );
  assert.equal(
    invoiceDuringFailure.activeClosureOperation?.operationId,
    accepted.operationId,
  );
  assert.equal(invoiceDuringFailure.activeClosureOperation?.status, "RETRYING");

  execFileSync(dockerExecutable, ["compose", "start", "inventory"], {
    stdio: "inherit",
  });
  inventoryRestored = true;
  await waitForInventory(30_000);

  const afterRecovery = await waitUntil(
    accepted.operationId,
    ({ status }) => status === "COMPLETED" || status === "FAILED",
    30_000,
  );
  const productAfter = await request(
    `/api/inventory/v1/products/${product.id}`,
  );
  const invoiceAfter = await request(`/api/billing/v1/invoices/${invoice.id}`);

  assert.equal(afterRecovery.status, "COMPLETED");
  assert.equal(productAfter.balance, 2);
  assert.equal(invoiceAfter.status, "CLOSED");
  assert.equal(invoiceAfter.activeClosureOperation, null);
  assert.ok(afterRecovery.attempts > duringFailure.attempts);

  console.log(
    JSON.stringify(
      {
        invoiceId: invoice.id,
        operationId: accepted.operationId,
        statusDuringFailure: duringFailure.status,
        attemptsDuringFailure: duringFailure.attempts,
        reloadResumesOperation: true,
        statusAfterRecovery: afterRecovery.status,
        attemptsAfterRecovery: afterRecovery.attempts,
        initialBalance: 5,
        finalBalance: productAfter.balance,
        invoiceStatus: invoiceAfter.status,
      },
      null,
      2,
    ),
  );
} finally {
  if (!inventoryRestored) {
    execFileSync(dockerExecutable, ["compose", "start", "inventory"], {
      stdio: "inherit",
    });
  }
}
