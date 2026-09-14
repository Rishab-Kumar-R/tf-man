CREATE TABLE IF NOT EXISTS products (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    price_cents INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Transactional outbox: recorded in the same DB transaction as the product
-- itself, so "the product exists" and "we intend to index it" can never
-- diverge. A relay loop delivers unindexed rows and retries until it
-- succeeds, instead of indexing synchronously inside the request path.
CREATE TABLE IF NOT EXISTS outbox (
    id UUID PRIMARY KEY,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    published BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Partial index: only covers unindexed rows, so the relay's poll query
-- stays fast regardless of how large the historical (indexed) table gets.
CREATE INDEX IF NOT EXISTS idx_outbox_unpublished ON outbox (created_at) WHERE published = false;
