CREATE SEQUENCE invoice_number_seq START WITH 1 INCREMENT BY 1;

CREATE TABLE invoices (
    id UUID PRIMARY KEY,
    number BIGINT NOT NULL DEFAULT nextval('invoice_number_seq'),
    status VARCHAR(16) NOT NULL CHECK (status IN ('OPEN', 'CLOSED')),
    version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at TIMESTAMPTZ NOT NULL,
    closed_at TIMESTAMPTZ,
    CONSTRAINT invoices_number_unique UNIQUE (number),
    CONSTRAINT invoices_closed_state_check CHECK (
        (status = 'OPEN' AND closed_at IS NULL) OR
        (status = 'CLOSED' AND closed_at IS NOT NULL)
    )
);

CREATE TABLE invoice_items (
    invoice_id UUID NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    product_id UUID NOT NULL,
    product_code VARCHAR(100) NOT NULL,
    product_description VARCHAR(500) NOT NULL,
    quantity BIGINT NOT NULL CHECK (quantity > 0),
    PRIMARY KEY (invoice_id, product_id)
);

CREATE TABLE closure_operations (
    id UUID PRIMARY KEY,
    invoice_id UUID NOT NULL REFERENCES invoices(id),
    idempotency_key VARCHAR(200) NOT NULL,
    command_id UUID NOT NULL,
    status VARCHAR(16) NOT NULL CHECK (status IN ('PENDING', 'PROCESSING', 'RETRYING', 'COMPLETED', 'FAILED')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at TIMESTAMPTZ NOT NULL,
    lease_until TIMESTAMPTZ,
    last_error VARCHAR(1000),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT closure_operations_idempotency_key_unique UNIQUE (idempotency_key),
    CONSTRAINT closure_operations_command_id_unique UNIQUE (command_id)
);

CREATE UNIQUE INDEX closure_operations_one_active_per_invoice
    ON closure_operations (invoice_id)
    WHERE status IN ('PENDING', 'PROCESSING', 'RETRYING');

CREATE INDEX closure_operations_worker_queue
    ON closure_operations (next_attempt_at, created_at)
    WHERE status IN ('PENDING', 'PROCESSING', 'RETRYING');
