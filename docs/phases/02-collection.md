ATLAS — Phase 2: Intelligence Collection Engine

Context

Extend the existing ATLAS application implemented in Phase 1.

Preserve its architecture, database, frontend and centralized scope validation service.

Implement a modular intelligence collection engine for authorized bug bounty targets.

Main objective

Transform publicly available and explicitly authorized reconnaissance observations into normalized, persistent intelligence.

Functional requirements

Collector architecture

Define a common collector interface supporting:

Collector identity.

Collection configuration.

Context cancellation.

Structured observations.

Error reporting.

Collectors must not write directly to the database. A centralized ingestion service must handle validation, normalization and persistence.

Initial collectors

Implement:

Certificate transparency collector using publicly available certificate records.

DNS collector using Go's networking libraries.

HTTP metadata collector for explicitly authorized hosts.

TLS certificate metadata collector.

Technology fingerprinting based on observable HTTP headers, metadata and public page content.

Passive collection must be the default.

Active HTTP and TLS collection must require explicit user authorization and scope validation.

Technology identification must distinguish confirmed observations from inferred technologies.

Collection orchestration

Implement a bounded worker pool supporting:

Queued collection runs.

Configurable concurrency.

Rate limiting.

Timeouts.

Cancellation.

Progress reporting.

Partial failures.

Structured logging.

Collection runs must have the following states:

Pending

Running

Completed

Partially completed

Failed

Cancelled

Validate scope before every active request, including redirects.

Never automatically expand active collection to related third-party infrastructure.

Ingestion service

Normalize collected information into entities, relationships and observations.

Prevent duplicate entities while preserving historical observations.

Record provenance for every discovery.

Store out-of-scope passive discoveries separately and never automatically perform active collection against them.

Input

A collection request containing:

{
  "target_id": "target-uuid",
  "collectors": [
    "certificate_transparency",
    "dns"
  ],
  "mode": "passive",
  "configuration": {
    "concurrency": 2,
    "timeout_seconds": 15
  }
}

The frontend must allow users to select collectors and review applicable scope restrictions before starting a run.

Output

Structured observations containing:

Entity type.

Entity value.

Related entity identifiers.

Relationship type.

Collector name.

Source.

Observation timestamp.

Confidence.

Evidence reference.

Scope classification.

Example observation:

{
  "entity_type": "subdomain",
  "value": "api.example.com",
  "source": "certificate_transparency",
  "scope_status": "in_scope",
  "confidence": "observed"
}

Expose REST endpoints to create, list, inspect and cancel collection runs.

Use Server-Sent Events for collection progress.

Frontend requirements

Implement:

Collection configuration dialog.

Collection history page.

Live progress display.

Asset inventory table.

Asset detail page.

Filtering by entity type and scope status.

Observation and evidence viewer.

Security and operational requirements

Enforce program-specific scope and restrictions.

Use conservative concurrency defaults.

Protect against requests to unauthorized private or local network destinations.

Validate redirects and resolved destinations.

Limit response sizes.

Treat collected external content as untrusted data.

Do not implement exploitation, credential attacks, brute-force discovery or authentication bypass.

Do not execute downloaded JavaScript.

Testing requirements

Use mock HTTP servers, mocked DNS responses and local fixtures.

Test:

Collector cancellation.

Rate limiting.

Scope enforcement.

Redirect validation.

Duplicate ingestion.

Partial failures.

Worker concurrency limits.

Observation provenance.

Automated tests must not depend on scanning external targets.

Acceptance criteria

Collection runs can be started from the web interface.

Results appear in the persistent asset inventory.

Progress is visible in real time.

Failed collectors do not corrupt existing intelligence.

Every observation has a source and timestamp.

Scope restrictions cannot be bypassed through collector configuration.

All tests pass.

Implement and test the complete phase. Update the README and API documentation
