CREATE TABLE IF NOT EXISTS orders (
      id UUID PRIMARY KEY,
      user_id TEXT NOT NULL,
      item_id TEXT NOT NULL,
      quantity INT NOT NULL,
      status TEXT NOT NULL,
      created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Transactional outbox: an event is recorded in the same DB transaction as
-- the state change it describes, so "did we save the order" and "did we
-- record intent to publish" can never diverge. A separate relay loop
-- delivers unpublished rows and retries until it succeeds.
CREATE TABLE IF NOT EXISTS outbox (
      id UUID PRIMARY KEY,
      event_type TEXT NOT NULL,
      payload JSONB NOT NULL,
      published BOOLEAN NOT NULL DEFAULT false,
      created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Partial index: only covers unpublished rows, so the relay's poll query
-- stays fast regardless of how large the historical (published) table gets.
CREATE INDEX IF NOT EXISTS idx_outbox_unpublished ON outbox (created_at) WHERE published = false;
