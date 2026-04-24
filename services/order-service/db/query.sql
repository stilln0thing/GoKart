-- name: CreateOrder :one
INSERT INTO orders (id, user_id, product_id, quantity, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, user_id, product_id, quantity, status, created_at, updated_at;

-- name: GetOrderByID :one
SELECT id, user_id, product_id, quantity, status, created_at, updated_at
FROM orders
WHERE id = $1;

-- name: UpdateOrderStatus :one
UPDATE orders
SET status = $2, updated_at = NOW()
WHERE id = $1
RETURNING id, user_id, product_id, quantity, status, created_at, updated_at;

-- name: UpsertUserView :exec
INSERT INTO users_view (id, username, email)
VALUES ($1, $2, $3)
ON CONFLICT (id) DO UPDATE
SET username = EXCLUDED.username,
    email = EXCLUDED.email;

-- name: UpsertProductView :exec
INSERT INTO products_view (id, name, price, quantity)
VALUES ($1, $2, $3, $4)
ON CONFLICT (id) DO UPDATE
SET name = EXCLUDED.name,
    price = EXCLUDED.price,
    quantity = EXCLUDED.quantity;

-- name: GetUserViewByID :one
SELECT id, username, email FROM users_view WHERE id = $1;

-- name: GetProductViewByID :one
SELECT id, name, price, quantity FROM products_view WHERE id = $1;
