ALTER TABLE products
    ADD CONSTRAINT products_safe_integers
    CHECK (
        balance <= 9007199254740991 AND
        version <= 9007199254740991
    );

ALTER TABLE stock_command_items
    ADD CONSTRAINT stock_command_items_quantity_safe_integer
    CHECK (quantity <= 9007199254740991);

ALTER TABLE stock_movements
    ADD CONSTRAINT stock_movements_safe_integers
    CHECK (
        quantity <= 9007199254740991 AND
        balance_before <= 9007199254740991 AND
        balance_after <= 9007199254740991
    );
