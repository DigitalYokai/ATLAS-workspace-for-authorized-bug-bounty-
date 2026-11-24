ATLAS — Phase 4: Intelligence Command Center

Context

Complete the ATLAS application implemented in Phases 1 through 3.

The application already contains target management, scope enforcement, collectors, persistent observations, intelligence correlation and investigation management.

This phase turns those capabilities into a cohesive investigation workspace.

Main objective

Allow users to explore, understand, maintain and export target intelligence through a professional web interface.

Functional requirements

Interactive knowledge graph

Use Cytoscape.js to visualize the existing intelligence graph.

Support:

Pan and zoom.

Node selection.

Relationship selection.

Entity type filtering.

Relationship type filtering.

Neighborhood expansion.

Search and highlighting.

Multiple graph layouts.

Node grouping.

Scope status visualization.

Selecting an entity must open a detail panel containing its metadata, evidence, observations and investigation status.

Do not load an entire large target graph by default. Retrieve bounded neighborhoods progressively.

Target dashboard

Display real database-derived metrics:

Total discovered entities.

In-scope entities.

Out-of-scope observations.

Entity counts by type.

Investigation coverage.

Recent collection runs.

Recent infrastructure changes.

Pending investigation notes.

Activity feed

Create a chronological feed containing:

New observations.

Collection events.

Infrastructure changes.

Manual investigation updates.

Scope modifications.

Allow filtering by activity type and date.

Scheduled collection

Implement persistent collection schedules with:

Target selection.

Collector selection.

Collection frequency.

Enabled or disabled status.

Last execution.

Next execution.

Execution history.

Scheduled collection must be passive by default.

Revalidate scope and program restrictions before every execution.

Prevent overlapping runs for the same schedule.

Provide a global mechanism to pause scheduled collection.

Notifications

Implement in-app notifications for:

Newly observed assets.

Infrastructure changes.

Collection failures.

Investigation follow-ups.

Notifications must be deduplicated and link to their supporting observations.

Reporting and export

Allow users to export:

Complete target intelligence as JSON.

Asset inventories as CSV.

Investigation summaries as Markdown.

Knowledge graph data as JSON.

Reports must contain timestamps, sources, scope information and confidence classifications.

Do not describe unverified observations as confirmed vulnerabilities.

Settings and operational security

Implement:

Local authentication.

Secure session management.

CSRF protection where applicable.

Secure password hashing.

Collection defaults.

Global concurrency limits.

Database backup and restore.

Data retention settings.

Keep the application bound to localhost by default unless remote access is explicitly configured.

Input

Graph exploration request:

{
  "target_id": "target-uuid",
  "root_entity_id": "entity-uuid",
  "depth": 2,
  "entity_types": ["subdomain", "ip_address", "technology"],
  "limit": 100
}

Scheduled collection request:

{
  "target_id": "target-uuid",
  "collectors": ["certificate_transparency", "dns"],
  "mode": "passive",
  "interval_hours": 24,
  "enabled": true
}

Output

Interactive target intelligence graph.

Persistent investigation workspace.

Target statistics and historical activity.

Scheduled collection results.

In-app notifications.

Exportable intelligence reports.

UI requirements

Maintain a consistent dark-first intelligence dashboard design.

Use:

Collapsible sidebar.

Target selector.

Global search.

Main workspace.

Contextual entity detail panels.

Keyboard-accessible controls.

Responsive layouts.

Avoid decorative visualizations that do not help investigations.

Use actual application data rather than hardcoded demonstration statistics.

Performance requirements

Paginate large inventories.

Use bounded graph queries.

Avoid unnecessary frontend requests.

Index frequently queried database fields.

Measure performance using synthetic datasets containing at least 10,000 entities.

Document observed performance rather than inventing benchmark results.

Testing requirements

Test:

Authentication and authorization.

Graph query limits.

Scheduled execution.

Schedule cancellation.

Scope revalidation.

Export correctness.

Backup and restore.

Notification deduplication.

Frontend navigation.

End-to-end target investigation workflow.

Acceptance criteria

A user can complete the following workflow entirely through the browser:

Create a target.

Configure authorized scope.

Start an intelligence collection.

Inspect discovered assets.

Explore their relationships.

Record investigation notes.

Compare historical observations.

Configure passive monitoring.

Export the target intelligence profile.

All tests must pass.

Provide complete deployment instructions, architecture documentation, API documentation and a user guide.

Review the existing implementation before modifying it. Complete the functionality rather than producing only a development plan.
