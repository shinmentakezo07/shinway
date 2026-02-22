# OpenAI-Compatible Providers UI Refresh Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Upgrade the OpenAI-compatible providers dashboard into a premium split-pane command-center interface while preserving all existing provider/key/alias/model functionality and API behavior.

**Architecture:** Keep existing data hooks, mutations, dialogs, and section responsibilities intact; reorganize page composition and styling around a command header, KPI strip, split-pane navigator/workspace, and refined tab workspace. Add only local UI helpers for presentation (counts, host extraction, class maps), avoid backend/API/schema changes, and preserve existing loading/error/empty paths.

**Tech Stack:** Next.js App Router, React 19, TypeScript, Tailwind CSS v4 utilities, existing internal UI components (`Card`, `Tabs`, `Button`, `Input`, `StatusBadge`), React Query via `useApi`.

---

### Task 1: Add a focused UI regression test harness for the page shell

**Files:**
- Create: `apps/ui/src/components/provider-keys/openai-compatible-providers-client.spec.tsx`
- Modify: `apps/ui/package.json` (add/align test script only if missing)
- Test: `apps/ui/src/components/provider-keys/openai-compatible-providers-client.spec.tsx`

**Step 1: Write the failing test**

```tsx
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { OpenAICompatibleProvidersClient } from "./openai-compatible-providers-client";

vi.mock("@/hooks/useDashboardNavigation", () => ({
	useDashboardNavigation: () => ({
		selectedOrganization: { id: "org_1", name: "Org" },
	}),
}));

vi.mock("@/lib/fetch-client", () => ({
	useApi: () => ({
		useQuery: () => ({
			isLoading: false,
			isError: false,
			data: {
				providers: [
					{
						id: "p1",
						organizationId: "org_1",
						name: "GatewayX",
						baseUrl: "https://api.gatewayx.dev/v1",
						status: "active",
						createdAt: "2026-01-01T00:00:00.000Z",
						updatedAt: "2026-01-01T00:00:00.000Z",
					},
				],
			},
		}),
		useMutation: () => ({
			isPending: false,
			mutateAsync: vi.fn(),
		}),
		queryOptions: () => ({ queryKey: ["providers"] }),
	}),
}));

describe("OpenAICompatibleProvidersClient layout", () => {
	it("renders command header and KPI strip", () => {
		render(<OpenAICompatibleProvidersClient />);
		expect(
			screen.getByRole("heading", { name: /OpenAI-Compatible Providers/i }),
		).toBeInTheDocument();
		expect(screen.getByText(/Total providers/i)).toBeInTheDocument();
		expect(screen.getByText(/Active providers/i)).toBeInTheDocument();
	});
});
```

**Step 2: Run test to verify it fails**

Run: `pnpm --filter ui test openai-compatible-providers-client.spec.tsx`
Expected: FAIL because test setup and/or KPI labels do not exist yet.

**Step 3: Add minimal test infra support**

- If `apps/ui` has no local test script, add one using Vitest + jsdom.
- Add only minimal config needed for this component-level regression test.

Example script snippet:

```json
{
	"scripts": {
		"test": "vitest run"
	}
}
```

**Step 4: Run test to verify harness works**

Run: `pnpm --filter ui test openai-compatible-providers-client.spec.tsx`
Expected: Test runner starts and produces deterministic pass/fail output.

**Step 5: Commit**

```bash
git add apps/ui/package.json apps/ui/src/components/provider-keys/openai-compatible-providers-client.spec.tsx
git commit -m "test(ui): add providers page layout regression harness"
```

---

### Task 2: Add presentation helpers for premium dashboard metadata

**Files:**
- Modify: `apps/ui/src/components/provider-keys/openai-compatible-providers-client.tsx`
- Test: `apps/ui/src/components/provider-keys/openai-compatible-providers-client.spec.tsx`

**Step 1: Write the failing test**

Add assertions for computed metadata:

```tsx
it("shows selected provider host and provider counts", () => {
	render(<OpenAICompatibleProvidersClient />);
	expect(screen.getByText(/api.gatewayx.dev/i)).toBeInTheDocument();
	expect(screen.getByText("1")).toBeInTheDocument();
});
```

**Step 2: Run test to verify it fails**

Run: `pnpm --filter ui test openai-compatible-providers-client.spec.tsx`
Expected: FAIL because host extraction/KPI content not yet implemented.

**Step 3: Write minimal implementation**

Add small local pure helpers in `openai-compatible-providers-client.tsx`:

```ts
function getProviderHost(baseUrl: string) {
	try {
		return new URL(baseUrl).host;
	} catch {
		return baseUrl;
	}
}

function countProvidersByStatus(providers: OpenAICompatibleProvider[]) {
	return {
		total: providers.length,
		active: providers.filter((p) => p.status === "active").length,
		inactive: providers.filter((p) => p.status === "inactive").length,
	};
}
```

Use these only for UI display; do not alter data flow.

**Step 4: Run test to verify it passes**

Run: `pnpm --filter ui test openai-compatible-providers-client.spec.tsx`
Expected: PASS for helper-driven rendering assertions.

**Step 5: Commit**

```bash
git add apps/ui/src/components/provider-keys/openai-compatible-providers-client.tsx apps/ui/src/components/provider-keys/openai-compatible-providers-client.spec.tsx
git commit -m "feat(ui): add providers dashboard metadata helpers"
```

---

### Task 3: Implement command header + KPI strip in the main client layout

**Files:**
- Modify: `apps/ui/src/components/provider-keys/openai-compatible-providers-client.tsx:1797-1922`
- Test: `apps/ui/src/components/provider-keys/openai-compatible-providers-client.spec.tsx`

**Step 1: Write the failing test**

Add shell assertions for KPI cards and context chip:

```tsx
it("renders command header context and KPI tiles", () => {
	render(<OpenAICompatibleProvidersClient />);
	expect(screen.getByText(/Configure provider base URLs/i)).toBeInTheDocument();
	expect(screen.getByText(/Selected provider/i)).toBeInTheDocument();
	expect(screen.getByText(/Inactive providers/i)).toBeInTheDocument();
});
```

**Step 2: Run test to verify it fails**

Run: `pnpm --filter ui test openai-compatible-providers-client.spec.tsx`
Expected: FAIL on missing header/KPI content.

**Step 3: Write minimal implementation**

Refactor top page block with:
- Premium command header container (layered card styles using existing tokens)
- KPI strip (4 compact cards)
- Keep `ProviderCreateDialog` wiring unchanged

Example JSX fragment:

```tsx
<div className="rounded-2xl border bg-card/80 p-5 shadow-sm">
	<h2 className="text-3xl font-bold tracking-tight">OpenAI-Compatible Providers</h2>
	<p className="text-muted-foreground">...</p>
</div>
<div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">...</div>
```

**Step 4: Run test to verify it passes**

Run: `pnpm --filter ui test openai-compatible-providers-client.spec.tsx`
Expected: PASS for header + KPI assertions.

**Step 5: Commit**

```bash
git add apps/ui/src/components/provider-keys/openai-compatible-providers-client.tsx apps/ui/src/components/provider-keys/openai-compatible-providers-client.spec.tsx
git commit -m "feat(ui): add command header and KPI strip"
```

---

### Task 4: Redesign provider navigator into premium left rail

**Files:**
- Modify: `apps/ui/src/components/provider-keys/openai-compatible-providers-client.tsx:1818-1890`
- Test: `apps/ui/src/components/provider-keys/openai-compatible-providers-client.spec.tsx`

**Step 1: Write the failing test**

Add assertions for enhanced list item metadata and selection label:

```tsx
it("shows provider host metadata in navigator", () => {
	render(<OpenAICompatibleProvidersClient />);
	expect(screen.getByText(/api.gatewayx.dev/i)).toBeInTheDocument();
	expect(screen.getByText(/Providers/i)).toBeInTheDocument();
});
```

**Step 2: Run test to verify it fails**

Run: `pnpm --filter ui test openai-compatible-providers-client.spec.tsx`
Expected: FAIL prior to navigator UI update.

**Step 3: Write minimal implementation**

Upgrade left rail while preserving selection logic:
- Scrollable list container
- Provider row as richer card/button
- Host line, status placement, selected-state ring/background
- Keep existing click handler and selected provider computation

Example class intent:

```tsx
className={cn(
	"group w-full rounded-xl border p-3 text-left transition",
	isSelected ? "border-primary/60 bg-primary/10" : "hover:bg-accent/60",
)}
```

Use existing utility patterns already used in repo (`cn` if available in this file’s imports).

**Step 4: Run test to verify it passes**

Run: `pnpm --filter ui test openai-compatible-providers-client.spec.tsx`
Expected: PASS with new metadata visible.

**Step 5: Commit**

```bash
git add apps/ui/src/components/provider-keys/openai-compatible-providers-client.tsx apps/ui/src/components/provider-keys/openai-compatible-providers-client.spec.tsx
git commit -m "feat(ui): refine provider navigator rail"
```

---

### Task 5: Polish provider details header and workspace shell

**Files:**
- Modify: `apps/ui/src/components/provider-keys/openai-compatible-providers-client.tsx:1677-1727`
- Test: `apps/ui/src/components/provider-keys/openai-compatible-providers-client.spec.tsx`

**Step 1: Write the failing test**

Add assertions for provider header metadata chips and stable action region:

```tsx
it("renders provider workspace header metadata", () => {
	render(<OpenAICompatibleProvidersClient />);
	expect(screen.getByText(/Created/i)).toBeInTheDocument();
	expect(screen.getByText(/GatewayX/i)).toBeInTheDocument();
});
```

**Step 2: Run test to verify it fails**

Run: `pnpm --filter ui test openai-compatible-providers-client.spec.tsx`
Expected: FAIL before header polish.

**Step 3: Write minimal implementation**

In `ProviderDetailsPane`:
- Add polished header shell with clear title/metadata grouping
- Keep update/delete actions and callbacks unchanged
- Improve spacing and card hierarchy only

Example:

```tsx
<CardHeader className="space-y-5 border-b bg-muted/20">
	...
</CardHeader>
```

**Step 4: Run test to verify it passes**

Run: `pnpm --filter ui test openai-compatible-providers-client.spec.tsx`
Expected: PASS for provider header assertions.

**Step 5: Commit**

```bash
git add apps/ui/src/components/provider-keys/openai-compatible-providers-client.tsx apps/ui/src/components/provider-keys/openai-compatible-providers-client.spec.tsx
git commit -m "feat(ui): polish provider workspace header"
```

---

### Task 6: Upgrade tabs workspace styling and table containers without behavior changes

**Files:**
- Modify: `apps/ui/src/components/provider-keys/openai-compatible-providers-client.tsx`
  - `ProviderKeysSection` (`~644-1093`)
  - `AliasesSection` (`~1099-1559`)
  - `ModelsSection` (`~1565-1669`)
  - `ProviderDetailsPane` tabs wrapper (`~1709-1724`)
- Test: `apps/ui/src/components/provider-keys/openai-compatible-providers-client.spec.tsx`

**Step 1: Write the failing test**

Add assertions for tab labels and empty-state copy stability:

```tsx
it("keeps keys aliases models tab workspace", () => {
	render(<OpenAICompatibleProvidersClient />);
	expect(screen.getByRole("tab", { name: /Keys/i })).toBeInTheDocument();
	expect(screen.getByRole("tab", { name: /Aliases/i })).toBeInTheDocument();
	expect(screen.getByRole("tab", { name: /Models/i })).toBeInTheDocument();
});
```

**Step 2: Run test to verify it fails**

Run: `pnpm --filter ui test openai-compatible-providers-client.spec.tsx`
Expected: FAIL if tab structure changed or labels regress.

**Step 3: Write minimal implementation**

Apply presentational updates only:
- Segment-like tab list visuals
- Consistent section headers, padding, and table wrappers
- Action button group consistency
- Enhanced empty-state containers

Do not change:
- Mutation/query calls
- form schemas and submit paths
- key/alias/model endpoint params

**Step 4: Run test to verify it passes**

Run: `pnpm --filter ui test openai-compatible-providers-client.spec.tsx`
Expected: PASS with tabs intact.

**Step 5: Commit**

```bash
git add apps/ui/src/components/provider-keys/openai-compatible-providers-client.tsx apps/ui/src/components/provider-keys/openai-compatible-providers-client.spec.tsx
git commit -m "feat(ui): refine tab workspace presentation"
```

---

### Task 7: Validate no functional regressions across lint/build

**Files:**
- Modify: none expected (only if fixes required)
- Test: repository-level checks

**Step 1: Write a failing verification checkpoint (if needed)**

If lint/build currently fails, capture exact failure output and create fix steps in-place before continuing.

**Step 2: Run verification commands**

Run:
- `pnpm format`
- `pnpm lint`
- `pnpm build`

Expected:
- format completes with no pending edits
- lint passes
- build passes for monorepo targets

**Step 3: Apply minimal fixes only if failures occur**

- Fix only issues introduced by this UI refresh.
- Re-run failed command until green.

**Step 4: Re-run full verification**

Run again:
- `pnpm lint`
- `pnpm build`

Expected: PASS.

**Step 5: Commit**

```bash
git add -A
git commit -m "chore(ui): verify providers dashboard refresh"
```

---

### Task 8: Final review checkpoint before integration

**Files:**
- Modify: none
- Test: optional visual/manual checks against local running UI

**Step 1: Run focused manual checks**

Run app:
- `pnpm dev`

Check page:
- `http://localhost:3002/dashboard/org/openai-compatible-providers`

Verify:
- Provider CRUD still works
- Key CRUD still works
- Alias CRUD still works
- Model search + refresh still works
- New layout is split-pane and visually premium

**Step 2: Capture evidence in summary notes**

Include exact observations and any edge cases validated.

**Step 3: Run final automated sanity**

Run:
- `pnpm --filter ui test openai-compatible-providers-client.spec.tsx`

Expected: PASS.

**Step 4: Prepare integration snapshot**

Run:
- `git status`
- `git log --oneline -n 5`

Expected: clean/expected state.

**Step 5: Commit (if any remaining changes)**

```bash
git add -A
git commit -m "docs(ui): finalize providers dashboard refresh checks"
```

(Only if there are remaining staged changes.)

---

## References

- UI component to refresh: `apps/ui/src/components/provider-keys/openai-compatible-providers-client.tsx`
- Existing page entrypoint: `apps/ui/src/app/dashboard/[orgId]/org/openai-compatible-providers/page.tsx`
- Existing global tokens: `apps/ui/src/app/globals.css`
- Approved design doc: `docs/plans/2026-02-22-openai-compatible-providers-ui-design.md`

## Notes for the Implementer

- Keep logic stable; prioritize visual hierarchy and management ergonomics.
- Avoid introducing new abstractions unless they remove immediate duplication in this file.
- Do not touch backend/gateway/db layers for this work.
- Keep commit titles <= 50 chars and conventional style.
