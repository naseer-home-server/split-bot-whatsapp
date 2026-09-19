ALTER TABLE splitbot_totals
    ADD COLUMN IF NOT EXISTS sheet_id VARCHAR,
    ADD COLUMN IF NOT EXISTS last_exported_at timestamptz;
