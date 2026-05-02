# Transaction Service

A production-quality transaction processing microservice built with Go, designed for Nigerian financial institutions with CBN compliance.

## Architecture

- **Go + Gin** REST API
- **Apache Kafka** for event streaming with DLQ support
- **PostgreSQL** for persistence with optimistic locking
- **WebSocket** for real-time transaction updates
- **NIBSS adapter** for interbank transfers
- **Apache Fineract adapter** for intrabank transfers
- **CBN compliance** audit logging

## Project Structure

```
├── cmd/server/          # Server entrypoint
├── internal/
│   ├── api/             # HTTP handlers, middleware, router
│   ├── adapters/        # NIBSS & Fineract integration clients
│   ├── compliance/      # CBN audit logging
│   ├── config/          # Environment-based configuration
│   ├── domain/          # Domain models and errors
│   ├── kafka/           # Producer, consumer, DLQ
│   ├── repository/      # Repository interfaces and PostgreSQL implementations
│   └── service/         # Business logic (transaction, hold, balance, transfer)
├── migrations/          # SQL migrations
└── tests/               # Unit and integration tests
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/transactions/deposit` | Deposit funds |
| `POST` | `/api/v1/transactions/withdrawal` | Withdraw funds |
| `POST` | `/api/v1/transactions/transfer` | Transfer between accounts |
| `POST` | `/api/v1/transactions/undo` | Reverse a transaction |
| `POST` | `/api/v1/transactions/adjust` | Balance adjustment |
| `GET` | `/api/v1/transactions/:id` | Get transaction by ID |
| `GET` | `/api/v1/transactions/account/:account_id` | List account transactions |
| `GET` | `/health` | Health check |
| `GET` | `/ws?account_id=<id>` | WebSocket connection |

All API endpoints require `Authorization: Bearer <token>` header.

## Getting Started

### Prerequisites

- Go 1.21+
- librdkafka-dev
- Docker & Docker Compose

### Local Development

```bash
# Copy environment config
cp .env.example .env

# Start dependencies
docker-compose up -d postgres zookeeper kafka

# Run migrations
psql $DATABASE_URL < migrations/001_initial_schema.sql
psql $DATABASE_URL < migrations/002_indexes.sql
psql $DATABASE_URL < migrations/003_cbn_settings.sql

# Build and run
CGO_ENABLED=1 go build -o transaction-service ./main.go
./transaction-service
```

### Docker

```bash
docker-compose up --build
```

### Testing

```bash
# Unit tests
CGO_ENABLED=1 go test ./tests/unit/... -v

# Integration tests (uses in-memory mocks)
INTEGRATION_TEST=1 CGO_ENABLED=1 go test ./tests/integration/... -v
```

## Key Features

- **Idempotency**: All write operations accept an idempotency key
- **Optimistic locking**: Prevents race conditions on account balance updates
- **Smart routing**: Intrabank transfers use Fineract; interbank uses NIBSS
- **Hold management**: Place and release balance holds
- **Retry with DLQ**: Kafka consumer retries with exponential backoff, failed messages go to DLQ
- **CBN audit trail**: All transactions logged for regulatory compliance (7-year retention)
- **WebSocket**: Real-time transaction event streaming per account
