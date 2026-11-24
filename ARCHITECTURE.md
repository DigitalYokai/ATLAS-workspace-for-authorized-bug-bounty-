# ATLAS — System Architecture Specification

## 1. Architectural vision

ATLAS is a self-hosted, web-based intelligence platform for authorized security research and bug bounty investigations.

Its central architectural responsibility is transforming fragmented observations into persistent, evidence-backed target knowledge.

Use a modular monolith with event-driven background processing.

Do not introduce microservices, Kubernetes, Redis, Kafka or an external graph database without a demonstrated requirement.

## 2. Technology stack

Backend:

- Go
- net/http
- encoding/json
- log/slog
- database/sql
- SQLite
- sqlc
- Goose

Frontend:

- React
- TypeScript with strict mode
- Vite
- Tailwind CSS
- shadcn/ui
- TanStack Query
- TanStack Table
- React Router
- Cytoscape.js
- Zod

Infrastructure:

- Docker
- Docker Compose
- Persistent database volume
- REST API
- Server-Sent Events

Testing:

- Go testing package
- httptest
- Vitest
- React Testing Library
- Playwright

## 3. Repository structure

```text
atlas/
├── AGENTS.md
├── ARCHITECTURE.md
├── DESIGN.md
├── README.md
├── Makefile
├── docker-compose.yml
├── .env.example
│
├── backend/
│   ├── cmd/
│   │   └── atlas/
│   │       └── main.go
│   ├── internal/
│   │   ├── target/
│   │   ├── scope/
│   │   ├── intelligence/
│   │   ├── observation/
│   │   ├── correlation/
│   │   ├── collection/
│   │   │   ├── collector/
│   │   │   │   ├── dns/
│   │   │   │   ├── ct/
│   │   │   │   ├── http/
│   │   │   │   └── tls/
│   │   │   ├── worker/
│   │   │   └── scheduler/
│   │   ├── investigation/
│   │   ├── reporting/
│   │   ├── notification/
│   │   ├── auth/
│   │   ├── event/
│   │   ├── platform/
│   │   │   ├── database/
│   │   │   ├── network/
│   │   │   ├── config/
│   │   │   └── logging/
│   │   └── transport/
│   │       └── http/
│   ├── db/
│   │   ├── migrations/
│   │   ├── queries/
│   │   └── generated/
│   └── go.mod
│
├── frontend/
│   ├── src/
│   │   ├── app/
│   │   ├── components/
│   │   │   ├── ui/
│   │   │   └── layout/
│   │   ├── features/
│   │   │   ├── targets/
│   │   │   ├── scope/
│   │   │   ├── assets/
│   │   │   ├── collection/
│   │   │   ├── graph/
│   │   │   ├── investigation/
│   │   │   └── reports/
│   │   ├── hooks/
│   │   ├── lib/
│   │   ├── routes/
│   │   ├── styles/
│   │   └── types/
│   └── package.json
│
└── docs/
    ├── api/
    └── decisions/
```

Organize backend code by business capability rather than creating global folders containing every model, service and repository.

Each module may contain its own model.go, service.go, repository.go and test files where necessary.

## 4. Dependency rules

Dependencies must flow inward toward business logic.

HTTP handlers depend on application services.

Application services depend on domain models and repository interfaces.

Infrastructure adapters implement those interfaces.

Domain models must not depend on HTTP, SQL, frontend concerns or third-party reconnaissance services.

Collectors must not directly access database repositories.

Avoid circular module dependencies.

Use explicit constructor-based dependency injection. Do not introduce a dependency injection framework.

The main package is responsible for assembling dependencies and starting the application.

## 5. Module responsibilities

### Target module

Owns target creation, metadata, lifecycle and program configuration.

### Scope module

Owns authorized scope rules, exclusions, permitted collection methods and scope decisions.

All active collection must use this module.

### Intelligence module

Owns canonical entities, typed relationships, graph queries and intelligence retrieval.

### Observation module

Owns immutable collection observations, evidence and provenance.

### Collection module

Owns collection runs, collector registration, job execution, cancellation, progress and rate limits.

### Correlation module

Owns deterministic rules that transform observations into entities and relationships.

### Investigation module

Owns notes, hypotheses, bookmarks, tags and investigation statuses.

### Reporting module

Owns export generation and historical intelligence summaries.

### Notification module

Owns persistent application notifications.

### Authentication module

Owns local user authentication and sessions.

## 6. Data model

Use a relational representation of a property graph.

Core database tables:

- targets
- scope_rules
- entities
- relationships
- observations
- evidence
- relationship_evidence
- collection_runs
- collection_jobs
- investigation_notes
- entity_tags
- tags
- snapshots
- schedules
- notifications
- outbox_events
- users
- sessions

All target-specific records must be isolated by target ID.

### Entities

Required fields:

- id
- target_id
- type
- canonical_value
- display_value
- metadata_json
- scope_status
- first_seen_at
- last_seen_at
- created_at
- updated_at

Enforce uniqueness on target_id, type and canonical_value.

### Relationships

Required fields:

- id
- target_id
- source_entity_id
- destination_entity_id
- relationship_type
- confidence
- first_seen_at
- last_seen_at
- status

Enforce uniqueness on target_id, source entity, destination entity and relationship type.

Prevent relationships between entities belonging to different targets.

### Observations

Required fields:

- id
- target_id
- collection_run_id
- collector
- observation_type
- subject
- value
- source
- evidence_id
- observed_at
- confidence
- raw_metadata_json

Observations represent historical facts and must not be silently overwritten.

### Evidence

Store:

- Evidence type.
- Source.
- Content hash.
- Sanitized content or controlled storage reference.
- Collection timestamp.
- Content size.

Apply configurable size and retention limits.

Never store unnecessary credentials, session tokens or sensitive personal information.

## 7. Intelligence processing pipeline

The processing lifecycle is:

1. User creates a collection run.
2. Application validates target configuration.
3. Jobs are persisted in the database.
4. Worker claims a job.
5. Collector performs permitted collection.
6. Collector emits structured observations.
7. Ingestion validates and normalizes observations.
8. Observations and internal events are persisted transactionally.
9. Correlation processes persisted observations.
10. Entities and relationships are updated.
11. Change events are generated.
12. Frontend receives progress notifications.

Ingestion and correlation must be idempotent.

A failed correlation operation must not destroy the original observation.

## 8. Collector contract

Define a small collector interface.

```go
type Collector interface {
    Name() string
    Collect(
        ctx context.Context,
        request CollectionRequest,
        emit func(Observation) error,
    ) error
}
```

CollectionRequest contains:

- Target identifier.
- Collection run identifier.
- Approved collection configuration.
- Execution mode.
- Time and resource limits.

Collectors receive controlled network dependencies through constructors.

The centralized network layer must enforce applicable scope decisions, redirect validation, rate limits and timeouts.

Collector output must be structured observations rather than direct database mutations.

## 9. Background processing

Use a persistent SQLite-backed job queue.

Support:

- Pending jobs.
- Running jobs.
- Completed jobs.
- Failed jobs.
- Cancelled jobs.
- Bounded retry attempts.
- Execution deadlines.
- Worker leases.
- Recovery after application restart.

Use a bounded worker pool.

Do not start unbounded goroutines for discovered assets.

Use an outbox table for internal events requiring reliable delivery.

Consumers must tolerate duplicate event delivery.

Server-Sent Events are a presentation mechanism, not the authoritative job state.

## 10. API conventions

Prefix all application endpoints with /api/v1.

Use plural resource names.

Example routes:

```text
GET    /api/v1/targets
POST   /api/v1/targets
GET    /api/v1/targets/{id}
PATCH  /api/v1/targets/{id}

GET    /api/v1/targets/{id}/scope
POST   /api/v1/targets/{id}/scope

GET    /api/v1/targets/{id}/entities
GET    /api/v1/targets/{id}/entities/{entityId}
GET    /api/v1/targets/{id}/graph

POST   /api/v1/targets/{id}/runs
GET    /api/v1/targets/{id}/runs
GET    /api/v1/runs/{id}
POST   /api/v1/runs/{id}/cancel
GET    /api/v1/runs/{id}/events
```

Use consistent JSON error responses.

Support pagination, validated filters and bounded query limits.

Use RFC 3339 UTC timestamps.

Use UUIDs for externally exposed identifiers.

Maintain an OpenAPI specification.

## 11. Frontend architecture

Organize frontend code by feature.

Use TanStack Query for server state.

Use local React state for temporary interface state.

Use URL search parameters for persistent filters, pagination and selected views.

Avoid a global state management library unless necessary.

Validate API responses at appropriate boundaries.

Keep API requests separate from presentation components.

Reuse shared layout and UI primitives.

## 12. Security architecture

Default to local-only deployment.

Require authentication before enabling remote access.

Implement scope validation as a mandatory backend capability, not merely a frontend check.

Distinguish passive discoveries from approved active collection destinations.

Protect against SSRF, DNS rebinding, unauthorized redirects and untrusted external content.

Apply resource limits to collection jobs and stored evidence.

Use parameterized database queries.

Implement secure sessions, CSRF protection where applicable and safe output encoding.

Do not execute collected scripts.

Do not implement automated exploitation or credential attacks.

## 13. Database operational requirements

Enable SQLite foreign key enforcement.

Use WAL mode and a suitable busy timeout.

Keep write transactions short.

Use controlled write concurrency.

Create indexes for target isolation, canonical entity lookup, graph traversal, observation history and job claiming.

Version all schema changes through Goose migrations.

Never manually modify generated sqlc files.

## 14. Testing strategy

Unit tests cover domain rules and normalization.

Integration tests cover repositories, ingestion, job processing and correlation.

HTTP tests cover validation, authentication and resource isolation.

Frontend tests cover feature behavior.

End-to-end tests cover complete browser workflows.

Use local fixtures and synthetic datasets.

External target scanning must not be required for automated tests.

## 15. Architecture governance

Record significant architectural decisions in docs/decisions using numbered ADR files.

Every ADR must contain:

- Context.
- Decision.
- Alternatives.
- Consequences.

Do not change the technology stack or architectural boundaries without documenting the reason.

Every development phase must preserve the constraints in this specification.
