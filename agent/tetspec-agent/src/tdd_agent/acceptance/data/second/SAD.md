# Payment gateway - architecture (synthetic fixture)

## 2. Decisions
| ID | Decision | Rationale | Alternatives | Tradeoffs |
|---|---|---|---|---|
| ADR-1 | PostgreSQL | transactional ledger with strong consistency | MongoDB, DynamoDB | operational skill |
| ADR-2 | Kafka | durable ordered event log | RabbitMQ | cluster operations |

## 3. Components
| ID | Name | Responsibility |
|---|---|---|
| SVC-1 | Capture service | Captures and refunds payments |
