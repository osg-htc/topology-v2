-- +goose Up
-- Disable is an independent field from Active in v1's YAML source data (real
-- resources exist with e.g. Active=false, Disable=false) -- v2 was fabricating
-- it as `!Active` at output time instead of storing it. Nullable, no default,
-- mirroring `active`'s own "preserve source absence" convention: an omitted
-- Disable must round-trip as omitted, not as a fabricated explicit false.
ALTER TABLE resources ADD COLUMN disable BOOLEAN;

-- +goose Down
ALTER TABLE resources DROP COLUMN disable;
