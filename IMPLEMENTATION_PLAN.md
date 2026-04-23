# GoKart Implementation Plan

This document outlines the step-by-step implementation plan for the GoKart Event-Driven Microservices Architecture, built using **Go, gRPC, Kafka, and PostgreSQL (via sqlc/pgx)**.

Every decision in this plan is made to be idiomatic Go and production-aware. The goal is not a demo — it's a system a senior engineer would take seriously.

---

## Phase 1: Local Infrastructure Setup (Docker Compose)

**Goal**: Establish the foundational infrastructure — databases, message broker, and topic bootstrapping.

**Tasks**:
- [ ] Create `docker-compose.yml` at the project root.
- [ ] Add a robust **Zookeeper & Kafka** setup for event messaging.
  - [ ] Include a one-shot `kafka-init` container that runs a shell script to create all required topics on startup: `user-registered`, `product-created`, `order-placed`, `inventory-updated`, `dlq-topic`.
- [ ] Add a central **PostgreSQL** container with a `docker-entrypoint-initdb.d` init script that creates three discrete logical databases: `userdb`, `productdb`, `orderdb`.
- [ ] Verify all required ports are exposed for local development connectivity.

**Go-Specific Decisions**:
- Use **sqlc** for type-safe SQL code generation against Postgres. Define `.sql` query files per service and generate Go code at build time.
- Use **pgx** as the underlying Postgres driver (sqlc supports pgx natively).
- Create a shared `pkg/db` package with a reusable `Connect(dsn string) (*pgxpool.Pool, error)` helper.

---

## Phase 2: User Service

**Goal**: Complete the baseline implementation of the `user-service` with proper proto definitions, sqlc-generated repo layer, and Kafka event production.

**Tasks**:
- [ ] Define `user.proto` with full lifecycle RPCs:
  - `RegisterUser` (unary)
  - `LoginUser` (unary) — returns a signed JWT.
  - `GetUser` (unary)
  - `UpdateUser` (unary)
- [ ] Generate Go code from proto definitions (`protoc` + `protoc-gen-go` + `protoc-gen-go-grpc`).
- [ ] Write SQL migrations and `query.sql` for the user table in `userdb`.
- [ ] Run `sqlc generate` to produce type-safe Go repository code.
- [ ] Implement the gRPC service handler, wiring sqlc-generated queries.
- [ ] Implement a Kafka producer (`segmentio/kafka-go`) — publish `UserRegistered` event on successful registration.
- [ ] **Graceful shutdown**: Handle `SIGTERM`/`SIGINT`, drain in-flight requests, close the Kafka producer and pgx pool cleanly.
- [ ] Validate end-to-end with `grpcurl` or Postman gRPC.

---

## Phase 3: Product Service

**Goal**: Scaffold and implement the product catalog and inventory management system.

**Tasks**:
- [ ] Define `product.proto`:
  - RPCs: `CreateProduct` (unary), `GetProduct` (unary), `UpdateInventory` (unary).
  - Messages: `Product`, `CreateProductRequest`, `InventoryUpdateRequest`, etc.
- [ ] Scaffold `product-service` following the clean workspace structure: `cmd/`, `domain/`, `service/`, `repository/`, `infra/`.
- [ ] Write SQL migrations and `query.sql` for the product table in `productdb`.
- [ ] Run `sqlc generate` for the product repo.
- [ ] Implement gRPC server on port `:50052`.
- [ ] Setup Kafka producer — publish `ProductCreated` and `InventoryUpdated` events.
- [ ] Setup Kafka consumer — subscribe to `order-placed` topic.
  - On receiving an `OrderPlaced` event, decrement product inventory via sqlc query.
  - Wrap consumer logic in error handling that routes failures to `dlq-topic` (see Phase 6).
- [ ] **Graceful shutdown**: Handle `SIGTERM`, drain Kafka consumer (commit offsets), close producer and pgx pool.

---

## Phase 4: Order Service & Event Choreography

**Goal**: Implement the `order-service` and integrate all microservices asynchronously via Kafka event choreography. Include at least one **server-streaming RPC**.

**Tasks**:
- [ ] Define `order.proto`:
  - RPCs:
    - `CreateOrder` (unary) — places an order, publishes `OrderPlaced`.
    - `GetOrder` (unary)
    - `StreamOrderStatus` (**server-streaming**) — streams real-time order status updates to the caller (e.g., `PLACED → CONFIRMED → SHIPPED`). This is the differentiating gRPC feature.
  - Messages: `Order`, `OrderStatusUpdate`, etc.
- [ ] Scaffold `order-service`, connect to `orderdb` via sqlc/pgx, expose gRPC on port `:50053`.
- [ ] Implement Kafka producer — publish `OrderPlaced` on order creation.
- [ ] Implement Kafka consumers:
  - Subscribe to `user-registered`, `product-created` for maintaining a local materialized view (eventual consistency / lightweight CQRS).
- [ ] Implement the `StreamOrderStatus` server-streaming RPC:
  - On order creation, write status transitions to a channel.
  - The streaming RPC reads from this source and sends `OrderStatusUpdate` messages to the client.
- [ ] **Graceful shutdown**: Handle `SIGTERM`, drain all Kafka consumers (commit offsets for each consumer group), close producers and pgx pool.

---

## Phase 5: gRPC Gateway Service

**Goal**: Provide a single edge endpoint for frontend clients by aggregating calls from backend gRPC services. **No GraphQL — pure gRPC gateway**.

> The GraphQL gateway adds complexity that doesn't contribute to the Go story. A gRPC gateway that aggregates downstream calls keeps the architecture diagram clean and the narrative tight.

**Tasks**:
- [ ] Create a `gateway-service` that exposes either:
  - **Option A**: A REST API using `grpc-gateway` (auto-generates REST endpoints from proto annotations). Good for browser/mobile clients.
  - **Option B**: A gRPC API that fans out to downstream services. Good for service-to-service or CLI clients.
  - Pick **Option A** (`grpc-gateway`) for maximum reach.
- [ ] Define `gateway.proto` with `google.api.http` annotations to map gRPC methods to REST routes.
- [ ] Implement gRPC client connections to `user-service`, `product-service`, and `order-service`.
- [ ] Implement request aggregation:
  - e.g., `GetOrderDetails` calls order-service, then enriches the response by calling user-service and product-service for nested data.
- [ ] Implement **JWT authentication middleware** at the edge — parse and validate tokens before forwarding to internal services.
- [ ] **Graceful shutdown**: Handle `SIGTERM`, close all outbound gRPC client connections cleanly.

---

## Phase 6: DLQ Service — Production-Grade Fault Tolerance

**Goal**: Build a proper Dead Letter Queue service that handles Kafka message failures with exponential backoff, retry tracking, and a dead-end logger. This is the feature that signals production thinking.

**Tasks**:
- [ ] Wrap all Kafka consumer `eachMessage` handlers across services in error handling:
  - On processing failure, increment `retry-count` in Kafka message headers.
  - Re-publish the failed message to `dlq-topic` with updated headers.
- [ ] Create a dedicated `dlq-service`:
  - [ ] Subscribe to `dlq-topic`.
  - [ ] On receiving a message:
    1. Read `retry-count` from headers.
    2. If `retry-count < MAX_RETRIES` (e.g., 5):
       - Wait using **exponential backoff**: `baseDelay * 2^retryCount` (with jitter).
       - Re-publish the message back to its **original topic** (stored in a header like `original-topic`).
       - Increment `retry-count` header.
    3. If `retry-count >= MAX_RETRIES`:
       - **Dead-end log**: Write the full message payload, headers, error context, and timestamp to a structured log file or stderr.
       - Do NOT retry. This message is now a dead letter requiring manual intervention.
  - [ ] Use **structured logging** (`log/slog` or `zerolog`) throughout the DLQ service for clean, parseable output.
- [ ] **Graceful shutdown**: Handle `SIGTERM`, commit final consumer offsets, flush any buffered logs.

---

## Phase 7: Structured Logging & Observability (TODO — Future Enhancement)

> *This phase is marked as a future TODO. Adding it takes a weekend and makes the project look 3x more serious.*

**Goal**: Instrument at least one service with structured logging and Prometheus metrics.

**Tasks**:
- [ ] Add **structured logging** across all services using `log/slog` (stdlib, Go 1.21+) or `zerolog`.
  - Replace all `fmt.Println` / `log.Println` calls with structured key-value log entries.
  - Log levels: `INFO` for happy paths, `WARN` for retries, `ERROR` for failures.
- [ ] Add **Prometheus metrics** to at least one service (e.g., `order-service`):
  - [ ] `grpc_request_duration_seconds` — histogram for gRPC handler latency.
  - [ ] `kafka_consumer_lag` — gauge for consumer group lag.
  - [ ] `kafka_messages_processed_total` — counter with labels for topic and status (success/failure).
  - [ ] `http_requests_total` — counter for gateway REST requests.
- [ ] Expose a `/metrics` endpoint using `prometheus/client_golang`.
- [ ] Optionally add a Grafana + Prometheus stack to `docker-compose.yml` for local dashboarding.

---

## Phase 8: README & Documentation

**Goal**: Write a proper README that a hiring manager or senior engineer reads *before* looking at any code.

**Tasks**:
- [ ] **Architecture diagram**: Mermaid or image showing all services, Kafka topics, Postgres databases, and the gateway.
- [ ] **How to run it**: `docker-compose up` one-liner, plus any prerequisites (Go version, protoc, sqlc).
- [ ] **Service breakdown**: One-paragraph description of each service and its responsibility.
- [ ] **Tradeoffs & decisions**: Document *why* you made each choice:
  - Why gRPC over REST internally?
  - Why sqlc over an ORM like GORM?
  - Why Kafka over RabbitMQ/NATS?
  - Why a gRPC gateway instead of GraphQL?
  - Why Postgres over MongoDB?
  - Why server-streaming for order status?
- [ ] **What's production-ready vs. what's not**: Be honest. Mention what you'd add with more time (e.g., distributed tracing, CI/CD, Kubernetes manifests).

---

## Cross-Cutting Concerns (Applied to Every Service)

These are not a separate phase — they must be baked into every service from Phase 2 onward:

| Concern | Implementation |
|---|---|
| **Graceful Shutdown** | Every service handles `SIGTERM`/`SIGINT` via `os/signal.NotifyContext`. Drain Kafka consumers, close pgx pools, stop gRPC servers with `GracefulStop()`. |
| **Structured Logging** | Use `log/slog` or `zerolog`. No raw `fmt.Println` in production code. |
| **Proto-first Design** | All service contracts are defined in `.proto` files under `proto/`. Code is generated, never hand-written. |
| **sqlc for DB Access** | All SQL queries are in `.sql` files. Go code is generated via `sqlc generate`. No hand-written query builders or ORMs. |
| **Error Handling** | Wrap errors with context using `fmt.Errorf("...: %w", err)`. Return proper gRPC status codes (`codes.NotFound`, `codes.Internal`, etc.). |
| **Config via Environment** | All config (DB DSN, Kafka brokers, ports) is read from environment variables. No hardcoded values. |


## Summary

New Structure
Phase 1-4: Same services, but every phase now includes graceful shutdown, sqlc, and structured logging as hard requirements
Phase 5: gRPC Gateway (replaces GraphQL)
Phase 6: Production-grade DLQ
Phase 7: Observability (marked TODO/future)
Phase 8: README & documentation
Cross-Cutting Concerns table: A reference for things that apply to every service