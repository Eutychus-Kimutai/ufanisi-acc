# Ufanisi Ledger

Event-driven ledger system for loans and investments.

## Overview

Ufanisi Ledger is a financial backend system that converts raw payment data into deterministic business state. It uses double-entry bookkeeping to maintain accurate financial records with full auditability.

The system operates across three conceptual layers:
- **Ingestion Layer** - Receives raw payment data
- **Processing Layer** - Interprets and classifies transactions
- **Ledger Layer** - Authoritative system of record with double-entry bookkeeping

## Architecture

The system consists of two main components that communicate via RabbitMQ:

**Ledger API** - A HTTP server that handles account and transaction operations. Clients interact with this service to create accounts, post transactions, and query balances. The API provides synchronous responses for real-time operations.

**Ledger Consumer** - A RabbitMQ consumer that processes transaction commands asynchronously. This service subscribes to message queues and executes ledger operations with built-in retry logic and dead-letter queue handling for failed messages.

The separation enables:
- Scalability through async processing
- Resilience through message persistence
- Decoupling between services

## Components

| Component | Description |
|-----------|-------------|
| **Ledger API** | HTTP server for account and transaction operations |
| **Ledger Consumer** | RabbitMQ consumer for async transaction processing |

## Features

- Double-entry bookkeeping (debits must equal credits)
- Transaction validation and atomic commits
- Account balance calculation
- Transaction history and audit trail
- Async processing via RabbitMQ
- Retry with exponential backoff
- Dead letter queue for failed messages

## Quick Start

### Prerequisites

- Go 1.25+
- PostgreSQL
- RabbitMQ

### Setup

1. **Clone the repository**
2. **Start PostgreSQL**
   ```bash
   brew services start postgresql
   ```
3. **Start RabbitMQ**
   ```bash
   brew services start rabbitmq
   ```
4. **Create the database**
   ```bash
   createdb ledger
   ```
5. **Configure environment**
   ```bash
   cp .env.example .env
   # Edit .env with your database credentials
   ```

### Run Services

Start the Ledger API:
```bash
go run cmd/ledger/main.go
```

Start the Ledger Consumer (in a separate terminal):
```bash
go run cmd/ledger-consumer/main.go
```

### Test the API

Create an account:
```bash
curl -X POST http://localhost:8080/accounts \
  -H "Content-Type: application/json" \
  -d '{"name": "Cash", "type": "Asset"}'
```

Post a transaction:
```bash
curl -X POST http://localhost:8080/transactions \
  -H "Content-Type: application/json" \
  -d '{
    "command_type": "POST_TRANSACTION",
    "payload": {
      "reference": "TEST-001",
      "entries": [
        {"account_id": "CASH-ACCOUNT-ID", "amount": 100, "type": "Debit"},
        {"account_id": "REVENUE-ACCOUNT-ID", "amount": 100, "type": "Credit"}
      ]
    }
  }'
```

## Configuration

### Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `DB_URL` | PostgreSQL connection string | `postgres://user:pass@localhost/ledger?sslmode=disable` |

### Config File

The `config.yaml` file configures RabbitMQ settings:

```yaml
rabbitmq:
  host: "localhost"
  port: 5672
  username: "guest"
  password: "guest"
  queues:
    loan: "ledger.loan"
    investment: "ledger.investment"
  retry:
    max_attempts: 5
    delay_seconds: 10
```

## Project Structure

```
.
├── cmd/
│   ├── ledger/              # HTTP API server entry point
│   └── ledger-consumer/     # RabbitMQ consumer entry point
├── internal/
│   ├── domain/              # Business logic and domain models
│   │   ├── types.go        # Account, Transaction, Entry types
│   │   └── ledger.go      # LedgerService with business rules
│   ├── repository/          # Data access layer
│   │   └── ledger.go      # Database operations via sqlc
│   ├── transport/          # HTTP transport layer
│   │   ├── handlers.go    # HTTP request handlers
│   │   └── router.go      # Route configuration
│   ├── commands/            # Message queue command types
│   │   └── types.go       # Command structures
│   └── rabbitmq/           # RabbitMQ utilities
│       ├── config.go      # Configuration loading
│       └── connection.go  # Connection and queue management
├── sql/
│   └── migrations/          # Database schema
├── config.yaml             # Application configuration
└── go.mod                  # Go module definition
```

## Data Flow

### HTTP Request Flow

1. Client sends HTTP request to Ledger API
2. Handler validates and parses request
3. LedgerService executes business logic
4. Repository persists to PostgreSQL
5. Response returned to client

### Async Flow via RabbitMQ

1. External system publishes command to RabbitMQ queue
2. Ledger Consumer receives message
3. Command is validated and decoded
4. LedgerService executes transaction
5. On success: message is acknowledged
6. On failure: retry with exponential backoff
7. After max retries: message moved to dead letter queue

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/accounts` | Create a new account |
| GET | `/accounts/:id` | Get account with balance |
| POST | `/transactions` | Post a transaction |
| GET | `/accounts/:id/transactions` | Get account transaction history |

## Message Queue

### Queues

| Queue | Purpose |
|-------|---------|
| `ledger.loan` | Loan transaction commands |
| `ledger.loan.dlq` | Failed loan commands |
| `ledger.investment` | Investment transaction commands |
| `ledger.investment.dlq` | Failed investment commands |

### Command Format

```json
{
  "command_type": "POST_TRANSACTION",
  "payload": {
    "reference": "TXN-001",
    "entries": [
      {"account_id": "uuid", "amount": 100, "type": "Debit"},
      {"account_id": "uuid", "amount": 100, "type": "Credit"}
    ]
  }
}
```

## Testing

Run unit tests:
```bash
go test ./...
```

Run integration tests:
```bash
# Ensure PostgreSQL and RabbitMQ are running
go test ./internal/domain/...
```

## License

MIT
