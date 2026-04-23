-- name: CreateProduct :one
INSERT INTO products (id, name, description, price, quantity, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, name, description, price, quantity, created_at, updated_at;

-- name: GetProductByID :one
SELECT id, name, description, price, quantity, created_at, updated_at
FROM products
WHERE id = $1;

-- name: UpdateInventory :one
UPDATE products
SET quantity = quantity + sqlc.arg(delta)::int,
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  -- Add a constraint check here so we don't drop below 0 if deducting
  AND (quantity + sqlc.arg(delta)::int) >= 0
RETURNING id, name, description, price, quantity, created_at, updated_at;
