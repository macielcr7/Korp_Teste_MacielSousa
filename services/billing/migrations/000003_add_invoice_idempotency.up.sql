ALTER TABLE invoices
    ADD COLUMN idempotency_key VARCHAR(200),
    ADD COLUMN request_hash CHAR(64),
    ADD CONSTRAINT invoices_idempotency_fields_check CHECK (
        (idempotency_key IS NULL AND request_hash IS NULL) OR
        (idempotency_key IS NOT NULL AND request_hash IS NOT NULL)
    );

CREATE UNIQUE INDEX invoices_idempotency_key_unique
    ON invoices (idempotency_key)
    WHERE idempotency_key IS NOT NULL;
