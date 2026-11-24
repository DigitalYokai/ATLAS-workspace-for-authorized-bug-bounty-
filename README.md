# ATLAS

ATLAS is a self-hosted intelligence workspace for authorized bug bounty research. It provides persistent target/scope management, authorized collection, and an evidence-backed Phase 3 intelligence graph for investigation.

## Run

```sh
cp .env.example .env
# edit .env and choose a unique password of at least 12 characters
docker compose up --build
```

Open http://localhost:5173. SQLite data is retained in the `atlas-data` Docker volume. The API is at http://localhost:8080 and is bound to localhost.

ATLAS now requires local login. The first administrator is created from `ATLAS_ADMIN_USERNAME` and `ATLAS_ADMIN_PASSWORD`; those values are only used to bootstrap an empty database. Use the command center to explore bounded knowledge-graph neighborhoods, configure passive DNS/CT monitoring, review activity, and export target intelligence.

## Development

```sh
cd backend && go test ./...
cd frontend && npm install && npm test && npm run build
```

## Architecture

The Go modular monolith separates target, scope, intelligence, investigation, platform, and HTTP transport concerns. SQLite migrations live in `backend/db/migrations`; queries are documented in `backend/db/queries` for sqlc generation. Scope decisions are centralized on the backend: exclusions override inclusions and label-safe wildcards never match a base domain.

Collection runs are persisted and expose progress through `/api/v1/runs/{id}/events` (SSE). Passive collection is the default. Active HTTP and TLS collectors require an approved scope method and reject private, loopback, link-local, multicast, and unspecified resolved destinations.

The collection queue is restart-safe: interrupted work is returned to the pending queue and claimed once when ATLAS starts again. Collection observations are idempotent within a run, retain collector/source/timestamp provenance, and feed the target asset inventory. The inventory supports entity type and scope filters, asset detail, and a safe evidence viewer.

## Correlation and investigation

Completed collection runs enqueue persistent correlation work. The engine creates only deterministic, evidence-backed relationships: DNS observations create `RESOLVES_TO`; CT observations create `HAS_SUBDOMAIN`; HTTP metadata creates `USES_TECHNOLOGY` and `HAS_ENDPOINT`; TLS metadata creates `PRESENTS_CERTIFICATE`. Relationships preserve supporting observations, confidence, first/last seen timestamps, and whether an assertion is observed, inferred, analyst-manual, or a hypothesis. Shared IP addresses never imply ownership.

Domains are normalized with host validation, IP addresses with canonical IP parsing, URLs by lower-casing the scheme and host while removing fragments/default ports, technologies by normalized whitespace, and certificate values by their collector-provided SHA-256 fingerprint. Entity search uses SQLite FTS and supports type, scope, investigation state, tag, technology, observation date, and relationship-confidence filters. Asset workspaces provide status, bookmarks, tags, evidence-backed timelines, manual observations, and explicit manual or hypothesis relationships; manual inputs are never presented as automated evidence.

Automatic snapshots are captured only after completed runs finish correlation. Manual snapshots are available for named baselines. An asset or relationship becomes historical only when absent from two consecutive completed automatic snapshots; failed, cancelled, and partial runs never mark intelligence as removed.
