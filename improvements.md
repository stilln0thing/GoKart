Currently in my architecture, the main problem is we have to do Dual Writes -- one for DB and one writing events to Kafka
Their might be a situation where DB write is successful but Kafka write fails or vice versa

To solve this we can use the Outbox Pattern

Outbox Pattern

In this pattern, we maintain an outbox table in our database. When we need to publish an event, we insert it into this outbox table along with the actual database transaction that modifies the business entity. After the transaction commits, a separate process (or a background job) reads from the outbox table and publishes the events to Kafka. If the Kafka publish fails, the event remains in the outbox table and can be retried later.