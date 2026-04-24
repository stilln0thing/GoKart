CREATE TABLE IF NOT EXISTS orders (
    id         UUID PRIMARY KEY,
    user_id    UUID         NOT NULL,
    product_id UUID         NOT NULL,
    quantity   INT          NOT NULL DEFAULT 1,
    status     VARCHAR(50)  NOT NULL DEFAULT 'PLACED',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_orders_status  ON orders(status);

-- Materialized view of users for the order service
CREATE TABLE IF NOT EXISTS users_view (
    id         UUID PRIMARY KEY,
    username   VARCHAR(100) NOT NULL,
    email      VARCHAR(255) NOT NULL UNIQUE
);

-- Materialized view of products for the order service
CREATE TABLE IF NOT EXISTS products_view (
    id          UUID PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    price       BIGINT       NOT NULL DEFAULT 0,
    quantity    INT          NOT NULL DEFAULT 0
);
