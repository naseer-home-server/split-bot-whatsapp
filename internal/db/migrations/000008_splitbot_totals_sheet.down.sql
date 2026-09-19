ALTER TABLE splitbot_totals
    DROP COLUMN IF EXISTS last_exported_at,
    DROP COLUMN IF EXISTS sheet_id;
