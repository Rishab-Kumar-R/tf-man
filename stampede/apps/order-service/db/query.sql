-- name: CreateOrder :one
INSERT INTO orders (id, user_id, item_id, quantity, status)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (id) DO UPDATE SET id = EXCLUDED.id
RETURNING *;

-- name: GetOrder :one
SELECT *
FROM orders
WHERE id = $1;

-- name: UpdateOrderStatus :exec
UPDATE orders SET status = $2 WHERE id = $1;

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
