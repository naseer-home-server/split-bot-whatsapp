CREATE TABLE IF NOT EXISTS splitbot_totals (
    id SERIAL PRIMARY KEY,
    group_id VARCHAR NOT NULL,
    poll_id INTEGER REFERENCES polls (id),
    items JSONB NOT NULL DEFAULT '[]'::jsonb,
    tax JSONB NOT NULL DEFAULT '[]'::jsonb,
    discount NUMERIC NOT NULL DEFAULT 0,
    total_in_bill NUMERIC NOT NULL,
    calculated_total NUMERIC NOT NULL,
    assignments JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_splitbot_totals_poll_id ON splitbot_totals (poll_id);
CREATE INDEX IF NOT EXISTS idx_splitbot_totals_group_id ON splitbot_totals (group_id);
