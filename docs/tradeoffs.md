# Trade-offs

- Top is kept in memory for low read latency.
- Service starts with empty data after restart.
- NATS Core is used for simple local deployment. Messages produced while the consumer is offline are not replayed.
- For production with replay guarantees, Kafka or NATS JetStream should be used with durable consumers.
- Anti-abuse is intentionally simple: per-user per-query limit per minute.
