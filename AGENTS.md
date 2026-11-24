# ATLAS — Engineering Instructions

## Product mission

ATLAS is a self-hosted target intelligence and investigation platform for authorized bug bounty research.

Its primary value is persistent, evidence-backed intelligence correlation.

## Engineering principles

- Prefer simplicity over premature abstraction.
- Maintain a modular monolith.
- Use Go idioms and the standard library whenever practical.
- Keep domain logic independent from HTTP handlers and database implementation.
- Use strict TypeScript.
- Preserve backward compatibility between development phases.
- Do not introduce unnecessary dependencies.
- Never silently discard errors.
- Use structured logging.
- Use context cancellation for background operations.
- Keep database migrations versioned.

## Intelligence integrity

- Every automated observation must have provenance.
- Preserve historical observations.
- Distinguish observed facts, inferred relationships and manual hypotheses.
- Never fabricate collection results.
- Normalize and deduplicate entities.
- Make ingestion and correlation idempotent.
- Treat external content as untrusted data.

## Scope and security

- Passive collection is the default.
- Active collection requires explicit authorization.
- Centralized scope validation is mandatory.
- Exclusions override inclusions.
- Never automatically expand authorized scope.
- Enforce rate limits, timeouts and cancellation.
- Do not implement automated exploitation or credential attacks.
- Do not store credentials or sensitive evidence in logs.
- Do not execute collected external content.

## Development workflow

Before implementing a feature:

1. Inspect the existing repository.
2. Identify relevant architecture and dependencies.
3. Define expected behavior and acceptance criteria.
4. Implement tests for critical behavior.
5. Implement the feature.
6. Run formatting, linting and tests.
7. Update documentation.

Do not claim completion if tests fail.

Do not remove existing functionality to make new functionality work.

Do not perform unrelated refactoring.

At the end of each implementation, report:

- Features implemented.
- Files created or modified.
- Architectural decisions.
- Tests executed and their actual results.
- Known limitations.
- Remaining work.
