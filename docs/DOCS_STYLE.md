# Documentation Standards

Purpose: keep project docs concise, accurate, and easy to scan.

## Scope For This Pass

- README.md
- docs/ARCHITECTURE.md
- docs/API.md
- docs/SETUP.md

## Length Targets

- README.md: 80-140 lines
- docs/ARCHITECTURE.md: <= 140 lines
- docs/API.md: <= 140 lines
- docs/SETUP.md: <= 140 lines

If a document exceeds target length, split details into a separate page instead of adding long sections.

## Section Pattern

Use this order where applicable:

1. What this page covers
2. Key tables or endpoint/command lists
3. Minimal executable examples
4. Source-of-truth references

## Writing Rules

- Prefer tables and bullets over long paragraphs.
- Keep examples short and runnable.
- Avoid repeating the same information across pages.
- Use exact names from code for routes, queues, keys, and service names.
- Mark known drift clearly with a short "Important" note.

## Source Of Truth

Use these files as authoritative references:

- internal/transport/router.go for Ledger API routes
- cmd/httpHandler/handler.go for worker HTTP endpoints
- cmd/\*/main.go for service startup and ports
- config.yaml for active runtime config values
- internal/rabbitmq/config.go for expected queue and retry config schema

## Update Checklist

Before merging doc changes:

1. Route parity: every route in internal/transport/router.go appears once in API docs.
2. Service parity: each active cmd/\*/main.go service appears in README or architecture docs.
3. Config parity: queues and retry keys in setup docs reflect config.yaml and note schema mismatches.
4. Command parity: run commands in docs start services on documented ports.

## Out Of Scope For This Pass

- Workflow deep dives
- Operations runbooks
- Documentation tooling setup (lint/site generation)
