-- =============================================================
-- GoKart — PostgreSQL Initialization Script
-- Creates three discrete logical databases for each microservice.
-- This script runs once on first container start.
-- =============================================================

-- User Service Database
CREATE DATABASE userdb;

-- Product Service Database
CREATE DATABASE productdb;

-- Order Service Database
CREATE DATABASE orderdb;

-- =============================================================
-- userdb schema
-- =============================================================
\c userdb;

CREATE TABLE IF NOT EXISTS users (
    id         UUID PRIMARY KEY,
    username   VARCHAR(100) NOT NULL,
    email      VARCHAR(255) NOT NULL UNIQUE,
    password   VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users(email);

-- =============================================================
-- productdb schema
-- =============================================================
\c productdb;

CREATE TABLE IF NOT EXISTS products (
    id          UUID PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    description TEXT         NOT NULL DEFAULT '',
    price       BIGINT       NOT NULL DEFAULT 0,
    quantity    INT          NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- =============================================================
-- orderdb schema
-- =============================================================
\c orderdb;

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
