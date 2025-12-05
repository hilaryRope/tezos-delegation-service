CREATE TABLE IF NOT EXISTS delegations (
    id BIGSERIAL PRIMARY KEY,
    tzkt_id BIGINT NOT NULL UNIQUE,
    timestamp TIMESTAMPTZ NOT NULL,
    amount BIGINT NOT NULL,
    delegator TEXT NOT NULL,
    level BIGINT NOT NULL,
    year INT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_delegations_timestamp_desc
    ON delegations (timestamp DESC);

CREATE INDEX IF NOT EXISTS idx_delegations_year_timestamp_desc
    ON delegations (year, timestamp DESC);
