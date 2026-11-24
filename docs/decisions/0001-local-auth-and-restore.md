# ADR 0001: Local authentication and operational safety

## Context

ATLAS is self-hosted and local-first but now offers persistent monitoring and intelligence exports.

## Decision

Use one administrator bootstrapped from environment variables, bcrypt password hashes, opaque database-backed sessions, HttpOnly SameSite cookies, and CSRF headers for mutations. Scheduled collection is passive-only and can be globally paused.

## Consequences

Deployments must supply credentials before first startup. Remote multi-user roles and external identity providers remain out of scope.
