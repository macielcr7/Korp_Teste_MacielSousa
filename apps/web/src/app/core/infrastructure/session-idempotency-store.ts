import { Injectable } from '@angular/core';

import { IdempotencyStore } from '../application/idempotency-store';

interface StoredIdentity {
  readonly key: string;
  readonly fingerprint?: string;
}

@Injectable({ providedIn: 'root' })
export class SessionIdempotencyStore implements IdempotencyStore {
  getOrCreate(scope: string, fingerprint?: string): string {
    const stored = sessionStorage.getItem(scope);
    if (stored) {
      const identity = this.parse(stored);
      if (identity && identity.fingerprint === fingerprint) return identity.key;
      if (fingerprint === undefined && identity === null) return stored;
    }

    const key = crypto.randomUUID();
    const value =
      fingerprint === undefined
        ? key
        : JSON.stringify({ key, fingerprint } satisfies StoredIdentity);
    sessionStorage.setItem(scope, value);
    return key;
  }

  clear(scope: string): void {
    sessionStorage.removeItem(scope);
  }

  private parse(value: string): StoredIdentity | null {
    try {
      const parsed = JSON.parse(value) as Partial<StoredIdentity>;
      return typeof parsed.key === 'string'
        ? { key: parsed.key, fingerprint: parsed.fingerprint }
        : null;
    } catch {
      return null;
    }
  }
}
