ALTER TABLE invoices
    ADD CONSTRAINT invoices_safe_integers
    CHECK (
        number <= 9007199254740991 AND
        version <= 9007199254740991
    );

ALTER TABLE invoice_items
    ADD CONSTRAINT invoice_items_quantity_safe_integer
    CHECK (quantity <= 9007199254740991);
