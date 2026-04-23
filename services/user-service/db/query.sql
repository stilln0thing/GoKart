-- name: CreateUser :one
INSERT INTO users (id, username, email, password, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, username, email, password, created_at, updated_at;

-- name: GetUserByID :one
SELECT id, username, email, password, created_at, updated_at
FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT id, username, email, password, created_at, updated_at
FROM users
WHERE email = $1;

-- name: UpdateUser :one
UPDATE users
SET username   = COALESCE(NULLIF(sqlc.arg(username)::text, ''), username),
    email      = COALESCE(NULLIF(sqlc.arg(email)::text, ''), email),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
RETURNING id, username, email, password, created_at, updated_at;
