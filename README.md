## Project HLD

graph TD
    subgraph "Client Layer"
        Web[Web/Mobile App]
    end

    subgraph "Gateway Layer"
        Gateway[GraphQL/gRPC Gateway]
    end

    subgraph "Service Layer (Go Microservices)"
        UserSvc[User Service]
        ProductSvc[Product Service]
        OrderSvc[Order Service]
    end

    subgraph "Persistence & Messaging"
        Postgres[(PostgreSQL)]
        Kafka{Kafka Bus}
    end

    %% Connections
    Web -->|JSON/GraphQL| Gateway
    Gateway -->|gRPC| UserSvc
    Gateway -->|gRPC| ProductSvc
    Gateway -->|gRPC| OrderSvc

    %% Events
    UserSvc -->|Publish: UserRegistered| Kafka
    ProductSvc -->|Publish: ProductUpdated| Kafka
    Kafka -->|Subscribe| OrderSvc
    
    %% DB
    UserSvc --- Postgres
    ProductSvc --- Postgres
    OrderSvc --- Postgres
