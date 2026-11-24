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

## Autonomous Execution Policy

Operate autonomously throughout the assigned implementation phase.

Read the relevant architecture, design and phase specifications before implementation.

Break the phase into actionable tasks and execute them sequentially.

Do not stop after completing individual tasks or milestones.

Do not request confirmation before ordinary implementation decisions.

Resolve compilation errors, failing tests, dependency issues and configuration problems independently.

Run tests and verification continuously.

Update documentation whenever implementation changes require it.

Treat progress reports as informational, not as requests for permission to continue.

Never substitute a proposed implementation plan for actual implementation.

Do not leave mandatory functionality unfinished merely because a substantial portion has been implemented.

Only stop and request user input when:

A fundamental architectural decision is required.

Requirements are contradictory.

Required credentials or external resources are unavailable.

An irreversible or destructive operation requires authorization.

A genuine blocker cannot be resolved independently.

Continue until every mandatory acceptance criterion for the assigned phase has been implemented and verified.

Do not proceed beyond the assigned phase unless explicitly authorized.

When finished, provide a consolidated report containing implemented functionality, test results, acceptance criteria and remaining limitations.
