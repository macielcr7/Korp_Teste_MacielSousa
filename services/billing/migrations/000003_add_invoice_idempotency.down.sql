DROP INDEX IF EXISTS invoices_idempotency_key_unique;

ALTER TABLE invoices
    DROP CONSTRAINT IF EXISTS invoices_idempotency_fields_check,
    DROP COLUMN IF EXISTS request_hash,
    DROP COLUMN IF EXISTS idempotency_key;
