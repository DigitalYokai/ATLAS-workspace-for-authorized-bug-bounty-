ATLAS — Phase 1: Application Foundation

Context

Build ATLAS, a self-hosted intelligence management platform for authorized bug bounty investigations.

ATLAS is not an exploitation framework. Its primary purpose is to maintain persistent, interconnected intelligence profiles of targets.

Build a working web application from the beginning. Do not implement a CLI.

Technology stack

Go with net/http for the backend.

React, TypeScript and Vite for the frontend.

Tailwind CSS and shadcn/ui.

SQLite for persistence.

sqlc for type-safe database queries.

Goose for database migrations.

Docker Compose for deployment.

Architecture requirements

Use a modular monolith with clear separation between:

Domain models.

Application services.

Database repositories.

HTTP handlers.

Frontend components.

Future intelligence collectors.

Prefer simple, maintainable architecture over unnecessary abstractions.

Suggested directory structure:

atlas/
  backend/
    cmd/server/
    internal/
      domain/
      service/
      repository/
      transport/http/
      collector/
      scope/
    db/
      migrations/
      queries/
  frontend/
    src/
      components/
      features/
      pages/
      services/
      types/
  docker-compose.yml
  README.md

Functional requirements

Target management

Users must be able to:

Create targets.

Edit targets.

Archive targets.

List targets.

Open individual target profiles.

Store the bug bounty platform and program URL.

Store program rules and investigation notes.

Scope management

Each target must support:

Explicitly allowed domains.

Wildcard domain rules.

Explicit exclusions.

Permitted investigation methods.

Program-specific restrictions and rate limits.

Implement a centralized scope validation service.

Exclusions must take precedence over inclusions. Wildcard matching must respect DNS label boundaries.

Unknown or ambiguous assets must be considered unapproved for active collection.

Intelligence data model

Create database models for:

Target

ScopeRule

Entity

Relationship

Observation

Evidence

CollectionRun

InvestigationNote

Supported initial entity types:

Domain

Subdomain

IP address

DNS record

Technology

URL

Service

Certificate

Entities must support extensible metadata.

Relationships must contain a source entity, destination entity and relationship type.

Observations must preserve source, timestamp, confidence and supporting evidence.

Use stable identifiers, foreign keys, indexes and uniqueness constraints.

Input

The target creation form must accept:

Name

Primary domain

Program platform

Program URL

Description

Scope rules

Program restrictions

Example:

{
  "name": "Example Program",
  "domain": "example.com",
  "platform": "HackerOne",
  "scope": {
    "include": ["*.example.com", "example.com"],
    "exclude": ["payments.example.com"]
  }
}

Output

A persistent target profile containing:

Target metadata.

Scope configuration.

Entity count.

Relationship count.

Observation count.

Investigation notes.

Collection history.

Expose REST endpoints for target management, scope management and read-only intelligence retrieval.

Frontend requirements

Implement:

Global dashboard.

Target listing page.

Target creation dialog.

Target overview page.

Scope management page.

Settings page.

Use a dark-first, professional intelligence dashboard aesthetic.

Include loading, empty, error and success states.

Do not populate production views with fabricated reconnaissance results.

Testing requirements

Write tests for:

Target creation.

Database persistence.

Scope validation.

Wildcard matching.

Exclusion precedence.

Duplicate entity prevention.

HTTP endpoint validation.

Acceptance criteria

The application starts using Docker Compose.

The user can create and manage targets through the browser.

Data survives application restarts.

Scope validation is centralized and tested.

Database migrations run successfully.

Backend and frontend tests pass.

README contains setup and architecture instructions.

Do not implement reconnaissance collectors yet.

Inspect the repository before making changes. Implement the application, run the tests and document architectural decisions. Do not stop at generating scaffolding or an implementation plan.
