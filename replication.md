E-Commerce Microservices Replication Guide
This document is a comprehensive technical breakdown of the eCommerce-Microservices codebase. Its purpose is to explain what is implemented and how it is implemented in meticulous detail so that you can replicate, modify, or draw inspiration from this architecture.

1. High-Level Architecture & Design Patterns
The codebase is built on an Event-Driven Architecture (EDA) utilizing multiple microservices. The services communicate with the outside world via an API Gateway (using GraphQL) and communicate with each other asynchronously via Message Broker (Kafka).

Core Patterns Used:
API Gateway Pattern: The graphql-gateway acts as the single entry point for clients, aggregating data from the underlying REST microservices (user-service, product-service, order-service) using HTTP/REST over Axios.
Database Per Service Pattern: Each microservice (user-service, product-service, order-service) has its own isolated MongoDB database instance. This guarantees loose coupling.
Event-Driven Choreography: Services emit domain events (like order-placed) to Kafka topics without knowing who will consume them. Subscribed services (like product-service) listen to these topics and take corresponding actions (like deducting inventory).
Dead Letter Queue (DLQ) Pattern: A dedicated dlq-service handles messages that fail processing after continuous retries.
2. Infrastructure Layer
All infrastructure and orchestration are defined in 
docker-compose.yml
.

Zookeeper & Kafka: Run as containerized services (confluentinc/cp-zookeeper and cp-kafka).
Kafka Topics Generator: A temporary initialization container that runs a bash script on startup to automatically create the necessary Kafka topics (user-registered, user-updated, product-created, order-placed, inventory-updated, dlq-topic).
MongoDB: Runs centrally locally (mongo:latest) mapped to port 27017, but each Node service hits a distinct logical database path:
User Service hits mongodb://mongodb:27017/userdb
Product Service hits mongodb://mongodb:27017/productdb
Order Service hits mongodb://mongodb:27017/orderdb
Docker Networking: All services resolve each other internally via Docker DNS (e.g., kafka:29092, user-service:5000).
3. Service Breakdown
All domain services are written in Node.js with Express.js, Mongoose (for MongoDB), and KafkaJS (for Kafka integration). Let's review each service deeply.

3.1. User Service
Role: Handles user identity, authentication, and profile metadata.

REST APIs: Provides endpoints like POST /api/users/register, POST /api/users/login, GET /api/users/find/:userId, PUT /api/users/update/:userId.
Database: Stores user records. The schema likely incudes fields like name, email, password (hashed using bcryptjs), and address.
Authentication: Uses jsonwebtoken (JWT). Upon /login, it validates hashed passwords and returns a signed JWT.
Kafka Events Emitted:
user-registered: Produced when a user successfully signs up.
user-updated: Produced when a user changes their profile details (e.g., address).
How to replicate: Create an Express server with Mongoose schemas for Users. Implement standard JWT login. Create a Kafka producer instance using kafkajs to .send() a message to user-registered inside the registration controller.
3.2. Product Service
Role: Manages catalog and inventory amounts.

REST APIs: Provides endpoints like POST /api/products/create, GET /api/products/getallproducts/all, GET /api/products/get/:productId, PUT /api/products/updateprice/:productId, PUT /api/products/updateinventory/:productId.
Database: Stores products (name, price, quantity, productId).
Kafka Events Emitted:
product-created: Informs other services about a new available catalog item.
inventory-updated: Informs downstream services that stock numbers have changed.
Kafka Events Consumed:
order-placed: CRITICAL LOGIC. When order-service fires order-placed, the Product service's consumer group picks it up, parses the payload (which contains productId and quantity), and automatically decrements the quantity in the productdb.
How to replicate: Model an Express+Mongoose server for products. Initialize a Kafka consumer with kafkajs subscribing to order-placed. In the .eachMessage consumer loop, execute a Mongoose findByIdAndUpdate to decrement stock based on the order quantity.
3.3. Order Service
Role: Manages order creation and stores order references.

REST APIs: POST /api/orders/create, GET /api/orders/getall, GET /api/orders/get/:orderId.
Database: Stores orders (containing user_Id, product_Id, quantity, order_Id). Note that to remain loosely coupled, it just stores the String IDs referring to users/products, relying on the GraphQL gateway to stitch/populate the full objects across microservices later.
Kafka Events Emitted:
order-placed: Fired heavily when a user makes a purchase. Contains the item purchased and the buyer's ID.
Kafka Events Consumed:
user-registered, user-updated, product-created, inventory-updated. The Order service listens to these to maintain localized eventual consistency (a materialized cached view of data it needs frequently).
How to replicate: You'll need both Kafka consumers to listen and update the local DB copies of User schemas/Product schemas (if doing CQRS/Materialized Views), and producers to trigger the order dispatch.
3.4. GraphQL Gateway
Role: The single point of entry connecting the frontend client to the backend REST array.

Tech Stack: Uses @apollo/server and axios.
How Data Aggregation Works: It does not have connecting databases. Instead, when a client sends a GraphQL query requesting an Order (and nest-requests details about the User and Product inside that order), the gateway:
Calls GET http://order-service:5001/api/orders/get/... via Axios to fetch the raw Order data.
Parses the userId attached to the order.
Plugs that ID into a second Axios request: GET http://user-service:5000/api/users/find/{userId} to fetch the user details.
Plugs the productId into a request to the product-service to fetch product data.
Combines (stitches) everything back together in the GraphQL Resolver and returns it as one clean JSON object to the client.
Security Check: The gateway checks the HTTP Authorization header, decodes the JWT token (or merely forwards it to the underlying services), protecting the inner microservices.
3.5. DLQ (Dead Letter Queue) Service
Role: Fault tolerance and error handling for Kafka messages.

How it handles failures: When product-service or order-service fail to process an incoming Kafka message (e.g. database goes offline temporarily when trying to decrement inventory), you don't want to lose the message.
The Retries: A typical DLQ setup handles failures by:
Catching errors in the main service consumer loop.
Relaying the exact message payload to a Kafka topic named dlq-topic.
The DLQ Service Logic: dlq-service subscribes to dlq-topic. It will wait (Exponential Backoff), and then re-inject the message as a producer back to the original topic. It keeps track of the header retry-count. If retry-count exceeds variables like MAX_RETRY_ATTEMPTS, it halts and logs the payload forcefully to an intervention dashboard or console, saving your system from infinite retry loops.
4. How to Replicate this Yourself (Step-by-Step)
If you are setting this up from scratch, follow this chronological order:

Phase 1: Local Foundation
Initialize a Monorepo/Multi-repo: Make a blank folder. Inside, generate 5 blank Node.js applications (npm init -y).
Infrastructure File: Create a 
docker-compose.yml
. You must define Zookeeper and Kafka first. Add a generic mongo container image. You don't need microservice Dockerfiles yet during active development.
Phase 2: Building Core REST Services
User Service: Set up basic Express routes. Add Mongoose. Define the User schema. Implement bcrypt hashing on the /register endpoint and jsonwebtoken signing on /login.
Product Service: Set up Express. Add a Product schema. Build endpoints to create products and fetch them.
Phase 3: Introducing Events (Kafka)
Implement kafkajs: In both User and Product services, initialize Kafka Client objects utilizing clientId and brokers.
Event Production: In your user.controller.js logic, directly after Mongoose successfully saves the user, fire producer.send({ topic: 'user-registered', messages: [{ value: JSON.stringify(newUser) }] }).
Phase 4: Downstream Complexity
Order Service: Set up Express and Mongoose order schema.
Event Consumption: Hook up a kafka.consumer({ groupId: 'product-group' }) inside your Product Service. Subscribe it to the order-placed topic. Update product inventory mathematically based on the payload.
Phase 5: GraphQL Unification
Set up Apollo: Boot up an Apollo Server inside the graphql-gateway folder.
Type Definitions: Rewrite all the REST data models into strict heavily-typed GraphQL schemas.
Resolvers Over Axios: Provide the mappings for TypeDefs using Axios HTTP GET calls mapping explicitly to your local microservice ports.
Phase 6: Edge Cases
DLQ Service: Setup a worker Node file. Connect a Kafka consumer to dlq-topic. If you wrap your main business logic in try-catch blocks and push to dlq-topic on catch, this service will retry firing them.


## go specific things

The architecture in that doc is solid but fairly standard. What will differentiate your version is the depth of Go-specific decisions you make. Some things that would make a senior engineer actually interested when reviewing your repo:

gRPC with proper proto definitions — not just unary RPCs, include at least one server-streaming use case (e.g. stream order status updates to the gateway).
Graceful shutdown everywhere — each service should handle SIGTERM, drain in-flight Kafka messages, close DB connections cleanly. Most student projects skip this entirely.
The DLQ service done properly — exponential backoff, retry count in Kafka headers, a dead-end logger. This is the thing that screams "I've thought about production".
// Structured logging + Prometheus metrics — instrument at least one service. Request latency, Kafka consumer lag, error rates. This takes a weekend to add and makes the project look 3x more serious. // TODO
sqlc or pgx over MongoDB — swap MongoDB for Postgres. Startups in India almost universally use Postgres. Using sqlc (type-safe SQL generation) specifically is a great talking point because it's very idiomatic Go.
A proper README with an architecture diagram, how to run it, and what tradeoffs you made. Engineers read this before the code.

What to cut from the original design:
The GraphQL gateway is extra complexity that doesn't add much to your Go story. Replace it with a simple gRPC gateway service that aggregates calls from the other services — keeps the architecture diagram clean and the story tighter.
