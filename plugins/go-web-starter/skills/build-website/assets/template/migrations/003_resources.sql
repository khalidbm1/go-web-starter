-- The empty key identifies the original module and preserves all existing records.
ALTER TABLE records ADD COLUMN resource_key text NOT NULL DEFAULT '';
CREATE INDEX records_resource_order ON records(owner_id, resource_key, created_at DESC, id DESC);
DROP INDEX records_owner_order;
