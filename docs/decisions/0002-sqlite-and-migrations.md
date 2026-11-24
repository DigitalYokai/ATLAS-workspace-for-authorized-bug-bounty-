# 0002: SQLite migrations

## Context
The product is self-hosted and must preserve historical intelligence.

## Decision
Use SQLite with WAL, foreign keys, Goose migrations, and sqlc query definitions.

## Alternatives
Unversioned schema creation; external database.

## Consequences
Deployments are simple while schema history remains explicit.
