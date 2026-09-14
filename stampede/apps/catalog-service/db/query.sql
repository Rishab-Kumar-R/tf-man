-- name: CreateProduct :one
INSERT INTO products (id, name, description, price_cents)
VALUES ($1, $2, $3, $4)
ON CONFLICT (id) DO UPDATE SET id = EXCLUDED.id
RETURNING *;

-- name: GetProduct :one
SELECT *
FROM products
WHERE id = $1;

-- name: InsertOutboxEvent :exec
INSERT INTO outbox (id, event_type, payload)
VALUES ($1, $2, $3)
ON CONFLICT (id) DO NOTHING;

-- name: ListUnpublishedOutboxEvents :many
SELECT *
FROM outbox
WHERE published = false
ORDER BY created_at
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: MarkOutboxEventPublished :exec
UPDATE outbox SET published = true WHERE id = $1;