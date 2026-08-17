CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX products_code_search_idx
    ON products USING GIN (lower(code) gin_trgm_ops);

CREATE INDEX products_description_search_idx
    ON products USING GIN (lower(description) gin_trgm_ops);

CREATE INDEX products_balance_code_idx
    ON products (balance, code, id);
