CREATE TABLE products (
    id UUID PRIMARY KEY,
    code VARCHAR(64) NOT NULL UNIQUE,
    description VARCHAR(255) NOT NULL,
    balance BIGINT NOT NULL CHECK (balance >= 0),
    version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT products_code_not_blank CHECK (btrim(code) <> ''),
    CONSTRAINT products_description_not_blank CHECK (btrim(description) <> '')
);

CREATE TABLE stock_commands (
    command_id UUID PRIMARY KEY,
    invoice_id UUID NOT NULL,
    payload_hash CHAR(64) NOT NULL,
    status VARCHAR(16) NOT NULL CHECK (status IN ('PROCESSING', 'COMMITTED', 'REJECTED')),
    error_code VARCHAR(64),
    error_detail VARCHAR(512),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    CONSTRAINT stock_commands_result_consistency CHECK (
        (status = 'PROCESSING' AND completed_at IS NULL AND error_code IS NULL) OR
        (status = 'COMMITTED' AND completed_at IS NOT NULL AND error_code IS NULL) OR
        (status = 'REJECTED' AND completed_at IS NOT NULL AND error_code IS NOT NULL)
    )
);

CREATE TABLE stock_command_items (
    command_id UUID NOT NULL REFERENCES stock_commands(command_id) ON DELETE RESTRICT,
    product_id UUID NOT NULL,
    quantity BIGINT NOT NULL CHECK (quantity > 0),
    PRIMARY KEY (command_id, product_id)
);

CREATE TABLE stock_movements (
    id UUID PRIMARY KEY,
    command_id UUID NOT NULL REFERENCES stock_commands(command_id) ON DELETE RESTRICT,
    invoice_id UUID NOT NULL,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    quantity BIGINT NOT NULL CHECK (quantity > 0),
    balance_before BIGINT NOT NULL CHECK (balance_before >= 0),
    balance_after BIGINT NOT NULL CHECK (balance_after >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT stock_movements_balance_transition CHECK (balance_after = balance_before - quantity),
    UNIQUE (command_id, product_id)
);

CREATE INDEX stock_movements_product_created_idx ON stock_movements (product_id, created_at DESC);
CREATE INDEX stock_commands_invoice_idx ON stock_commands (invoice_id);
