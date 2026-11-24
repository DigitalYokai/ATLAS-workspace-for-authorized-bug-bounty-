# ATLAS — Design System and UX Specification

## 1. Design philosophy

ATLAS is an intelligence workstation, not a marketing website.

The interface must prioritize:

- Information density.
- Rapid navigation.
- Technical precision.
- Evidence visibility.
- Investigation continuity.
- Consistent interaction patterns.

Visual references include Linear, Obsidian, GitHub Dark and professional security research tools.

Do not imitate their branding.

Avoid generic SaaS dashboards, excessive whitespace, decorative gradients, glassmorphism, oversized cards and unnecessary animations.

The application should feel like a serious engineering instrument.

## 2. Visual identity

Product name: ATLAS

Subtitle: Attack Surface Intelligence System

Use a minimal crosshair, compass or geometric network symbol as the product icon.

Use uppercase ATLAS branding with restrained letter spacing.

Dark mode is the default appearance.

## 3. Color tokens

```css
:root {
  --background: #0B0D10;
  --surface: #12161C;
  --surface-elevated: #1A2029;
  --border: #29313D;
  --primary: #5B8DEF;
  --foreground: #E6EDF3;
  --muted: #8B949E;

  --success: #3FB950;
  --warning: #D29922;
  --danger: #F85149;
  --inferred: #A78BFA;
  --unknown: #6B7280;
}
```

Map these tokens into the Tailwind and shadcn/ui theme.

Do not hardcode arbitrary colors in individual feature components.

Primary blue is used for navigation, selection and primary actions.

Green represents successful operations or explicitly approved scope.

Amber represents warnings, incomplete information or attention requirements.

Red represents errors, excluded scope or destructive operations.

Purple represents inferred intelligence.

Gray represents unknown or neutral states.

Do not use color as the only indicator of meaning.

## 4. Typography

Use Inter for general interface text.

Use JetBrains Mono for:

- Domains.
- IP addresses.
- URLs.
- HTTP headers.
- Identifiers.
- Technical metadata.
- Raw observations.
- Code.
- Relationship types.

Typography scale:

- 11px: compact metadata.
- 12px: secondary text.
- 13px: dense table content.
- 14px: standard interface text.
- 16px: section headings.
- 20px: page headings.
- 24px: major numerical metrics.

Avoid oversized headings.

Use tabular numerals for metrics and timestamps.

## 5. Spacing and surfaces

Use a 4px spacing system.

Preferred spacing values:

4, 8, 12, 16, 20, 24 and 32px.

Use border radii between 4px and 8px.

Use subtle one-pixel borders to separate panels.

Avoid heavy shadows.

Use compact controls suitable for desktop investigation workflows.

Standard table rows should be approximately 36px high, with an optional comfortable density mode.

## 6. Application shell

The desktop application uses four structural regions:

1. Global sidebar.
2. Top navigation bar.
3. Main workspace.
4. Contextual inspector.

### Global sidebar

Expanded width: 224px.

Collapsed width: 56px.

Contains:

- ATLAS branding.
- Global dashboard.
- Targets.
- Global search.
- Collection activity.
- Notifications.
- Settings.

Within a selected target, expose target-specific navigation.

### Top navigation

Height: approximately 48px.

Contains:

- Breadcrumbs.
- Current target selector.
- Search trigger.
- Collection status.
- Notification indicator.

### Main workspace

Uses remaining available width.

Supports tables, graphs, timelines and forms.

Avoid unnecessary maximum-width restrictions on technical data views.

### Contextual inspector

Default width: 340px.

Resizable where practical.

Displays selected entity information without forcing the user to leave the current investigation.

Contains:

- Entity identity.
- Scope status.
- Metadata.
- Relationships.
- Observations.
- Evidence.
- Notes.
- Investigation status.

### Status bar

Height: approximately 24px.

Displays application version, connection status and background collection activity.

## 7. Navigation structure

```text
Dashboard
Targets
  Target overview
  Scope
  Assets
  Knowledge graph
  Observations
  Collection runs
  Changes
  Investigation notes
  Reports
Activity
Settings
```

Use stable, bookmarkable routes.

Preserve filters and selected entities through URL parameters where practical.

## 8. Target overview page

Display:

- Target identity and program metadata.
- Scope configuration summary.
- Asset count.
- Investigation coverage.
- Recent observations.
- Collection history.
- Infrastructure changes.
- Pending follow-ups.

Metrics must be derived from actual persisted data.

Do not use decorative charts without analytical value.

## 9. Asset inventory

The asset inventory is a primary investigation interface.

Use TanStack Table.

Support:

- Sorting.
- Filtering.
- Pagination.
- Column visibility.
- Column resizing.
- Search.
- Row selection.
- Keyboard navigation.

Default columns:

- Asset.
- Entity type.
- Scope status.
- Technology.
- Investigation status.
- First observed.
- Last observed.
- Source.

Selecting a row opens the contextual inspector.

Use monospace typography for technical values.

Long values should truncate visually but remain accessible through expansion and copying.

## 10. Knowledge graph

Use Cytoscape.js.

The graph should occupy the available workspace.

Use a subtle grid background.

Node visual semantics:

- Domain: blue.
- Subdomain: blue.
- IP address: green.
- Technology: purple.
- URL: neutral gray.
- Certificate: amber.
- Service: cyan.

Use icons or shapes in addition to colors.

Observed relationships use solid edges.

Inferred relationships use dashed edges.

Historical relationships use reduced opacity.

Show relationship direction where meaningful.

Provide:

- Zoom.
- Pan.
- Fit to view.
- Search.
- Entity filtering.
- Relationship filtering.
- Neighborhood expansion.
- Layout selection.
- Reset view.

Selecting a node opens the contextual inspector.

Do not render the entire graph automatically for large targets.

Avoid continuous decorative graph animations.

## 11. Collection interface

Provide a collection configuration dialog.

Display:

- Target.
- Approved scope.
- Available collectors.
- Collection mode.
- Applicable restrictions.
- Rate limits.
- Execution configuration.

Make passive and active collection modes visually distinguishable.

Require explicit confirmation for active collection.

Display live collection progress through structured status information.

Do not create fake terminal output for visual effect.

## 12. Observation and evidence viewer

Use a structured key-value interface.

Support:

- Source inspection.
- Timestamp inspection.
- Raw evidence expansion.
- Copying technical values.
- Related entity navigation.
- Confidence classification.

Render raw external content as escaped text.

Never render collected HTML directly in the application context.

Clearly distinguish observations, inferences and hypotheses.

## 13. Investigation workspace

Support persistent investigation notes, tags, bookmarks and hypotheses.

Use a Markdown editor with preview.

Allow notes to reference entities and observations.

Investigation statuses:

- Unreviewed.
- Reviewing.
- Reviewed.
- Needs follow-up.
- Archived.

Do not equate reviewed status with a secure or vulnerability-free asset.

## 14. Component standards

Create reusable components for:

- ApplicationShell.
- Sidebar.
- Breadcrumbs.
- TargetSelector.
- PageHeader.
- DataTable.
- EntityBadge.
- ScopeIndicator.
- ConfidenceIndicator.
- InvestigationStatus.
- MetricCard.
- EntityInspector.
- EvidenceViewer.
- CollectionProgress.
- ActivityTimeline.
- EmptyState.
- ErrorState.
- ConfirmDialog.
- CommandPalette.

Use Lucide icons consistently.

Avoid introducing multiple competing component libraries.

## 15. Interaction rules

Use predictable interactions throughout the application.

Single click selects an entity.

Explicit navigation opens the full entity page.

Escape closes dialogs and temporary panels.

Enter submits appropriate forms.

Ctrl/Cmd + K opens the command palette.

Ctrl/Cmd + F may activate workspace search when appropriate without unnecessarily overriding browser behavior.

Provide keyboard shortcuts only when they improve investigation workflows.

Use visible focus indicators.

Use confirmation dialogs for destructive operations.

## 16. Loading and feedback

Use skeleton states for initial data loading.

Use inline progress for long-running operations.

Preserve existing content during background refreshes.

Show actionable error messages.

Avoid blocking the entire workspace during collection.

Use transient notifications for successful actions and persistent notifications for events requiring attention.

## 17. Empty states

Every data-driven view must support an empty state.

An empty asset inventory should explain that no assets have been observed and provide access to collection configuration.

An empty graph should explain that relationships have not yet been collected.

An empty investigation page should provide an action to create a note.

Do not populate empty production views with fabricated intelligence.

## 18. Responsive behavior

Desktop is the primary environment.

Support desktop widths of 1280px and above.

At smaller widths:

- Collapse the sidebar.
- Convert the inspector into a drawer.
- Allow horizontal table scrolling.
- Preserve access to all critical actions.

Mobile should support reviewing intelligence and notes, even if complex graph exploration is less comfortable.

## 19. Accessibility

Target WCAG 2.2 AA where applicable.

Provide accessible labels.

Ensure sufficient text contrast.

Do not communicate status through color alone.

Support keyboard navigation.

Respect reduced-motion preferences.

## 20. Design acceptance criteria

Every implemented page must:

- Use the shared design tokens.
- Use the shared application shell.
- Support loading, empty and error states.
- Preserve consistent typography and spacing.
- Display actual application data.
- Support keyboard interaction.
- Avoid introducing unrelated visual patterns.

All future development phases must follow this specification.
