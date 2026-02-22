# OpenAI-Compatible Providers UI Redesign

Date: 2026-02-22
Owner: UI
Status: Approved for planning

## Objective

Redesign the OpenAI-compatible providers management page to be visually stronger and easier to operate, while preserving all existing behavior and API interactions.

Target page:
- `apps/ui/src/components/provider-keys/openai-compatible-providers-client.tsx`

## Constraints

- Preserve all existing functionality:
  - Provider CRUD
  - Provider key CRUD
  - Alias CRUD
  - Model discovery and refresh
- Preserve existing endpoint usage, mutation/query behavior, and debounced search behavior.
- Keep compatibility with current dashboard shell and theme tokens.
- Keep mobile/tablet usability.

## Selected Direction

**Split-pane Command Center (Premium control-center aesthetic)**

Why this direction:
- Best for high-frequency management workflows.
- Improves scanability and task-switch speed.
- Fits existing component structure with low behavioral risk.

## Information Architecture

### 1) Command Header

- Large page title + supporting description.
- Primary CTA: Add Provider (existing dialog component).
- Context chips for organization/active provider.
- Subtle premium backdrop treatment (non-distracting).

### 2) KPI Strip

Compact summary cards:
- Total providers
- Active providers
- Inactive providers
- Selected provider summary (status + host)

### 3) Split Workspace

#### Left Rail: Provider Navigator
- Search input at top.
- Provider list with richer row cards:
  - Name
  - Host extracted from base URL
  - Status badge
  - Strong selected-state styling
- Loading/error/empty states remain, but visually refined.

#### Right Pane: Provider Workspace
- Sticky provider identity header:
  - Name, base URL, status, created timestamp
  - Edit and delete actions
- Tabs for Keys / Aliases / Models remain primary workspace structure.

## Component Strategy

Reuse and restyle existing functional components:

- `ProviderCreateDialog`
- `ProviderUpdateDialog`
- `ProviderDeleteButton`
- `ProviderKeysSection`
- `AliasesSection`
- `ModelsSection`

No API contract changes. No mutation/query key strategy changes.

## Interaction Design

- Provider selection behavior unchanged (selected or fallback to first provider).
- Newly created provider remains auto-selected.
- Deletion behavior remains safe with selection reset/re-resolution.
- Tab interaction remains the same; only visual treatment changes.

## Visual System

- Premium enterprise style: layered card surfaces, soft borders, restrained accent usage.
- Stronger hierarchy via spacing and typography scale.
- Consistent action groups and table affordances.
- Minimal, purposeful motion (hover/focus/active/loading only).

## Responsiveness

- Desktop: true split-pane.
- Tablet/mobile: stacked panels.
- Tabs and action clusters remain usable in constrained widths.

## Accessibility

- Preserve full keyboard accessibility.
- Improve focus visibility on provider navigator rows and icon-only actions.
- Keep `sr-only` labels for icon buttons.
- Maintain contrast via existing theme variables.

## Error/Loading/Empty States

- Keep existing logic paths and messages where applicable.
- Improve visual containers and actionability of empty states.
- Preserve destructive styling for failures.

## Testing and Verification

### Functional
- Provider create/update/delete works unchanged.
- Key create/update/delete works unchanged.
- Alias create/update/delete works unchanged.
- Model search + refresh works unchanged.

### UI/UX
- Selected provider state is always visually clear.
- Header + KPI + split-pane hierarchy is consistent.
- Mobile and tablet layouts remain navigable.

### Regression
- No endpoint or payload changes.
- No query invalidation regressions.
- No broken dialog open/close/reset flows.

## Non-Goals

- No backend/API changes.
- No schema/migration updates.
- No changes to permission/auth behavior.

## Acceptance Criteria

- Page is visually upgraded to a premium control-center style.
- Existing management workflows remain intact end-to-end.
- Usability improves for both first-time and frequent operators.
