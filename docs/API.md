# API Reference

## What This Page Covers

- Ledger API endpoints
- Minimal request examples
- Worker HTTP endpoints

## Base URL

- Local: http://localhost:8080

## Endpoints

| Method | Path                        | Description                      |
| ------ | --------------------------- | -------------------------------- |
| GET    | /accounts/{id}/transactions | List transactions for an account |
| GET    | /accounts/{id}              | Get account details and balance  |
| POST   | /accounts                   | Create a new account             |
| POST   | /transactions               | Post a transaction command       |
| POST   | /investments                | Create investment                |
| POST   | /withdrawals                | Request investment withdrawal    |

## Minimal Examples

Create account:

```bash
curl -X POST http://localhost:8080/accounts \
  -H "Content-Type: application/json" \
  -d '{"name":"Cash","type":"Asset"}'
```

Post transaction:

```bash
curl -X POST http://localhost:8080/transactions \
  -H "Content-Type: application/json" \
  -d '{
    "command_type": "POST_TRANSACTION",
    "payload": {
      "reference": "TXN-001",
      "entries": [
        {"account_id": "UUID-1", "amount": 100, "type": "Debit"},
        {"account_id": "UUID-2", "amount": 100, "type": "Credit"}
      ]
    }
  }'
```

## Worker HTTP Endpoints

Loan and investment workers expose:

| Method | Path     | Purpose              |
| ------ | -------- | -------------------- |
| POST   | /payment | Ingest payment event |
| GET    | /health  | Liveness check       |

Default worker ports:

- Loan worker: http://localhost:8081
- Investment worker: http://localhost:8082

## Notes

- Endpoint source of truth: internal/transport/router.go
- Worker endpoint source: cmd/httpHandler/handler.go
