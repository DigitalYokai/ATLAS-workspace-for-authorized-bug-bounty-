# ATLAS

ATLAS is a self-hosted intelligence workspace for authorized bug bounty research. Phase 1 provides persistent target and scope management; it does not perform reconnaissance or active collection.

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
