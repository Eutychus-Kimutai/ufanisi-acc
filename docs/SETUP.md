# Setup

## What This Page Covers

- Prerequisites and env vars
- RabbitMQ config keys
- Local run order
- Health checks and quick verification

## Prerequisites

- Go 1.21+
- PostgreSQL
- RabbitMQ

## Environment Variables

| Name   | Required | Description                     |
| ------ | -------- | ------------------------------- |
| DB_URL | Yes      | PostgreSQL DSN used by services |

Example:

```bash
export DB_URL='postgres://user:pass@localhost/ledger?sslmode=disable'
```

## RabbitMQ Configuration

Configuration file: config.yaml

Current keys:

```yaml
host: localhost
port: 5672
username: guest
password: guest
queues:
  loan: ledger.loan
  investment: ledger.investment
  unresolved: payment.unresolved
retry:
  max_attempts: 5
  delay_seconds: 10
```

Important: code expects retry.max_retries while config.yaml currently uses retry.max_attempts.

## Run Services Locally

Terminal 1:

```bash
go run cmd/ledger/main.go
```

Terminal 2:

```bash
go run cmd/ledger-consumer/main.go
```

Terminal 3:

```bash
go run cmd/investment/main.go
```

Loan worker note:

- `cmd/loan_worker/main.go` currently declares `package loanworker`, so `go run cmd/loan_worker/main.go` is not executable until that package is changed to `main`.

## Health Checks

- Ledger API: call a known endpoint such as GET /accounts/{id}
- Loan worker (after package fix): GET http://localhost:8081/health
- Investment worker: GET http://localhost:8082/health

## Verification Checklist

- Services can connect to PostgreSQL using DB_URL
- Services can connect to RabbitMQ using config.yaml
- Ledger API responds on port 8080
- Worker health endpoints respond on ports 8081 and 8082
- Retry config key is aligned with internal/rabbitmq/config.go
