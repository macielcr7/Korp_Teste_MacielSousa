/** Application port for browser-resilient idempotency keys. */
export abstract class IdempotencyStore {
  abstract getOrCreate(scope: string, fingerprint?: string): string;
  abstract clear(scope: string): void;
}
