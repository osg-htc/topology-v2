-- +goose Up
-- ResourceGroup.Disable was hardcoded to false in /rgsummary/xml instead of
-- being read from real data -- v1 defaults it to false but honors an explicit
-- override in the RG's own YAML (11 real resource groups set it explicitly
-- today, all to false). Nullable, no default, mirroring resources.disable's
-- own "preserve source absence" convention.
ALTER TABLE resource_groups ADD COLUMN disable BOOLEAN;

-- +goose Down
ALTER TABLE resource_groups DROP COLUMN disable;
