# ATLAS

ATLAS is a self-hosted intelligence workspace for authorized bug bounty research. It provides persistent target/scope management and a Phase 2 collection engine for permitted DNS and Certificate Transparency collection, plus explicitly authorized HTTP/TLS metadata collection.

## Run

```sh
docker compose up --build
```

Open http://localhost:5173. SQLite data is retained in the `atlas-data` Docker volume. The API is at http://localhost:8080 and is bound to localhost.

## Development

```sh
cd backend && go test ./...
cd frontend && npm install && npm run build
```

## Architecture

The Go modular monolith separates target, scope, intelligence, investigation, platform, and HTTP transport concerns. SQLite migrations live in `backend/db/migrations`; queries are documented in `backend/db/queries` for sqlc generation. Scope decisions are centralized on the backend: exclusions override inclusions and label-safe wildcards never match a base domain.

Collection runs are persisted and expose progress through `/api/v1/runs/{id}/events` (SSE). Passive collection is the default. Active HTTP and TLS collectors require an approved scope method and reject private, loopback, link-local, multicast, and unspecified resolved destinations.
