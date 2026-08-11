# CLIProxyAPI Feature Port Plan

## Purpose

Port the useful capabilities present in `router-for-me/CLIProxyAPI` into this
repository without replacing this repository with the reference project.

Reference snapshot used for this plan:

- Repository: `https://github.com/router-for-me/CLIProxyAPI`
- Revision: `ecc9aa72b32f34b680d03b0724b531a21ae74472`

Local baseline at planning time:

- Revision: `7e6ce1e`

This is a staged implementation plan. It is not authorization to overwrite
local code with matching reference files.

## Non-Negotiable Preservation Rules

1. Do not reset, checkout, clean, or overwrite the working tree.
2. Preserve the existing uncommitted `.gitignore` entry for `shinway`.
3. Preserve local-only functionality unless a later approved change explicitly
   replaces it:
   - NVIDIA NIM provider and `internal/thinking/provider/nvidia`.
   - OpenCode Zen provider.
   - Local management web panel and API integration.
   - Persistent usage analytics under `internal/usagestore` and `db/`.
   - Local Docker and Compose panel build and persistence behavior.
4. Prefer new files and narrow additive hooks. When a shared file must change,
   apply the smallest compatible edit after re-reading its current version.
5. Do not copy module paths, branding, SDK package renames, deployment files,
   sponsors, or reference-only assets.
6. Each batch must compile and pass focused tests before the next batch starts.
7. Keep every cache and session boundary scoped by model, credential, and
   downstream caller identity where the reference feature handles reasoning,
   signatures, or other hidden state.

## Porting Method

For every batch:

1. Re-fetch or refresh the reference clone in `/tmp/CLIProxyAPI-shinway-compare`.
2. Read the local target files immediately before editing. Do not rely on old
   diffs because local work may have changed between batches.
3. Identify the behavior rather than mechanically copying reference code.
4. Translate imports from `github.com/router-for-me/CLIProxyAPI/v7/...` to
   `github.com/shinmentakezo07/shinway/v7/...`.
5. Map `cliproxy` SDK types to the local `shinway` SDK only when their current
   interfaces are semantically compatible. If they are not, adapt the feature
   rather than changing the SDK architecture as a side effect.
6. Add focused unit or HTTP/SSE integration tests with each feature.
7. Run `gofmt` only on files changed in the batch.
8. Run targeted packages first, then `go test ./...` when the batch changes a
   common package or cross-provider translation behavior.
9. Inspect `git diff --check`, `git diff`, and `git status --short` before
   closing the batch. Confirm the `.gitignore` change remains intact and no
   local-only files were deleted.

## Dependency and Risk Summary

| ID | Capability | Depends on | Risk | Recommended order |
| --- | --- | --- | --- | --- |
| A | Baseline safety and comparison fixtures | None | Low | 0 |
| B | Claude compatible thinking replay | C, existing Claude/Codex helpers | High | 1 |
| C | Bounded replay cache primitives and Kimi replay | Existing Home KV | Medium-High | 2 |
| D | Translator model/tool identity correctness | None | Medium | 3 |
| E | Claude consecutive-message accumulator | D optional | Medium | 4 |
| F | Kimi signature validation and compatibility | C | Medium | 5 |
| G | Codex reasoning replay and stale-signature recovery | Existing Codex cache, websocket paths | High | 6 |
| H | API-key model metadata and context capabilities | D optional | High | 7 |
| I | Codex API-key Alpha Search routing | H | High | 8 |
| J | Claude OAuth safety and request hardening | Existing Claude auth | Medium-High | 9 |
| K | Claude native-client compatibility helpers | J, B | High | 10 |
| L | Provider-neutral reasoning summary intent | F, local NVIDIA merge | Medium-High | 11 |
| M | Registry catalog update | Existing registry | Medium | 12 |
| N | SDK/Home execution registry architecture | Separate design approval | Very High | 13 |
| O | SDK documentation | N or verified existing APIs | Low | 14 |

The order is intentional. The early batches are concrete provider behavior and
translator correctness. The Home/SDK architecture is deferred because it is a
large migration, not a safe feature-level port.

## Phase 0: Baseline Safety and Test Harness

### Goal

Establish reproducible comparison and validation without modifying product
behavior.

### Work

1. Record the local HEAD, reference revision, `go version`, and test baseline.
2. Confirm the local tree contains only expected user changes before edits.
3. Keep the reference clone outside the repository in `/tmp`; never add it as a
   Git remote, submodule, vendored directory, or production dependency.
4. Create a batch checklist in pull-request or implementation notes showing:
   changed files, tests, known deviations from reference, and rollback steps.
5. Use package-level test commands throughout the port:

   ```sh
   go test ./internal/cache ./internal/runtime/executor
   go test ./internal/translator/...
   go test ./internal/config ./internal/registry ./internal/api/...
   go test ./...
   ```

### Acceptance criteria

- `.gitignore` remains modified only by the existing user line.
- No untracked reference-clone files appear in the repository.
- Current local NVIDIA, Zen, management panel, and usage-store tests remain
  runnable before feature work starts.

## Phase 1: Claude Compatible Thinking Replay

### Goal

Support multi-turn Claude API-key compatible sessions when clients omit signed
thinking blocks from historical assistant tool-use turns.

### Reference behavior

- Dedicated state cache:
  `internal/cache/claude_thinking_replay_cache.go`.
- Request/replay orchestration:
  `internal/runtime/executor/claude_thinking_replay.go`.
- Non-stream hook:
  `internal/runtime/executor/claude_executor_execute.go`.
- Stream hook:
  `internal/runtime/executor/claude_executor_stream.go`.

### Local targets

- Add `internal/cache/claude_thinking_replay_cache.go`.
- Add `internal/cache/claude_thinking_replay_cache_test.go`.
- Add `internal/runtime/executor/claude_thinking_replay.go`.
- Add `internal/runtime/executor/claude_thinking_replay_test.go`.
- Narrowly update `internal/cache/signature_cache.go` to expire the new cache.
- Narrowly update `internal/runtime/executor/claude_executor_execute.go`.
- Narrowly update `internal/runtime/executor/claude_executor_stream.go`.

### Implementation details

1. Enable only when all conditions hold:
   - source format is Claude;
   - selected provider is Claude;
   - credential is an API key, not OAuth;
   - requested model is explicitly marked compatible;
   - API key is non-empty and is not an OAuth token.
2. Derive scope from:
   - suffix-stripped model name;
   - a hash of the selected credential identity or endpoint identity;
   - existing execution/session-key precedence;
   - downstream API-key isolation for caller-controlled session keys.
3. Do not create a shared fallback scope when session identity or downstream API
   key is unavailable. Disable replay for that request.
4. Store complete assistant `content` arrays, preserving signed `thinking` and
   matching `tool_use` blocks.
5. Before request translation, restore a cached assistant array only when:
   - the request contains a matching assistant turn;
   - non-thinking blocks match canonically and in order;
   - the current turn does not already carry thinking or redacted thinking.
6. Restore more than one historical assistant turn when separate cached turns
   match separate request turns.
7. Persist only replayable completed turns: at minimum a signed thinking block
   paired with a tool-use block. A completed non-replayable turn conditionally
   clears stale replay state.
8. For non-stream responses, read `content` from the final Claude response.
9. For SSE, reconstruct blocks from start, delta, stop, and final message-stop
   events. Do not persist malformed, upstream-error, cancelled, incomplete, or
   partially consumed streams.
10. When replay was injected and upstream returns an invalid-request style 400
    or 422, conditionally clear only the snapshot used by that request.

### Cache contract

- TTL: one hour with refresh-on-read.
- Maximum local entries: 10,240.
- Maximum turns per session: 64.
- Maximum bytes per session: 8 MiB.
- Maximum blocks per turn: 512.
- Maximum aggregate local bytes: 256 MiB.
- Evict the oldest 128 entries when a limit is exceeded.
- Home KV keys must hash model family and session key.
- Home KV state must use CAS snapshots and tombstones so a stale request cannot
  overwrite newer data or resurrect an absent-state race.

### Tests

1. Eligibility rejects OAuth, non-Claude source, non-Claude provider,
   non-compatible model, and missing API key.
2. Non-stream two-turn replay restores a missing signed thinking block.
3. SSE two-turn replay restores the same state after consuming stream chunks.
4. Multi-turn history restores multiple omitted assistant turns.
5. Existing thinking is never duplicated.
6. A mismatching assistant tool-use turn is never rewritten.
7. Invalid upstream replay request clears only the matching snapshot.
8. Complete non-replayable response clears older replay state.
9. Cache enforces TTL, per-session, total-memory, block, and turn limits.
10. Home KV CAS rejects stale writes and deletes.

### Validation

```sh
go test ./internal/cache -run ClaudeThinkingReplay
go test ./internal/runtime/executor -run ClaudeThinkingReplay
go test ./internal/runtime/executor -run ClaudeExecutor
go test ./...
```

## Phase 2: Kimi Thinking Replay and Shared Bounded Cache Support

### Goal

Add equivalent signed-thinking continuity for Kimi and extract only genuinely
shared stream-reconstruction helpers from Phase 1.

### Reference sources

- `internal/cache/kimi_thinking_replay_cache.go`
- `internal/runtime/executor/kimi_thinking_replay.go`
- `internal/runtime/executor/kimi_executor.go`
- optional `internal/cache/bounded_lru.go`

### Local targets

- Add Kimi replay cache and tests.
- Add Kimi replay executor logic and tests.
- Add small request/response hooks to local `kimi_executor.go`.
- Add cache cleanup registration to `signature_cache.go`.

### Implementation details

1. Reuse the safe session isolation approach from Claude replay.
2. Keep Kimi and Claude cache namespaces separate.
3. Preserve Kimi-specific validation and replayability rules. Do not force
   Claude signature assumptions onto Kimi signatures.
4. Use a shared stream accumulator only if it is provider-neutral and covered
   by tests; otherwise retain provider-specific wrappers.
5. Require signed thinking and tool use for a replayable Kimi assistant turn.
6. Use CAS generations/tombstones in Home KV and generation checks locally.

### Tests and validation

```sh
go test ./internal/cache -run KimiThinkingReplay
go test ./internal/runtime/executor -run KimiThinkingReplay
go test ./internal/runtime/executor -run Kimi
```

## Phase 3: Translator Correctness Foundation

### Goal

Port low-level translator correctness fixes before adding more provider-specific
behavior.

### Subphase 3A: Caller model identity

Add a shared helper equivalent to reference `internal/translator/common/request.go`.

Behavior:

1. Prefer the user-supplied model from `opts.OriginalRequest`.
2. Fall back to the translated request model.
3. Support wrapper shapes such as `request.model`.
4. Preserve caller-facing aliases in Responses output rather than leaking
   upstream resolved model names.

### Subphase 3B: Responses tool-call identity

Add a shared helper equivalent to reference
`internal/translator/common/responses.go`.

Behavior:

1. Set `name` and `namespace` consistently for function/custom tool calls.
2. Remove stale names/namespaces when absent.
3. Support item-root and nested response item paths.

### Local files requiring manual merge

- `internal/translator/codex/openai/responses/*`
- `internal/translator/openai/openai/responses/*`
- `internal/translator/gemini/openai/responses/*`
- `internal/translator/openai/interactions/responses/*`
- `internal/translator/claude/openai/responses/*`

### Tests

- Caller alias survives each affected response path.
- Tool names/namespaces survive and stale fields are removed.
- Tool-call assistant content is preserved while converting Responses events.
- Existing local translator benchmarks still compile and run.

### Validation

```sh
go test ./internal/translator/common
go test ./internal/translator/...
```

## Phase 4: Claude Message Accumulator

### Goal

Make Claude request translations safe for upstream alternating-message rules
without losing tool, thinking, cache-control, or content ordering semantics.

### Reference sources

- `internal/translator/common/claude_messages.go`
- request translators for OpenAI Chat, Gemini, and Interactions to Claude.

### Implementation details

1. Add a `ClaudeMessageAccumulator` in local translator common code.
2. Coalesce adjacent messages with the same role.
3. Keep system handling compatible with existing local top-level-system logic.
4. Keep block-level cache controls intact.
5. Place assistant text/thinking content before `tool_use` blocks.
6. Skip empty messages without collapsing valid role boundaries.
7. Update only affected request translators after proving equivalent output for
   representative existing tests.

### Tests and validation

- Adjacent same-role messages coalesce.
- Assistant tool blocks remain last.
- Thinking and cache-control blocks survive.
- Empty content does not create illegal turns.

```sh
go test ./internal/translator/common
go test ./internal/translator/claude/...
```

## Phase 5: Kimi Signature Validation and Provider Compatibility

### Goal

Avoid forwarding foreign, malformed, or unsupported thinking signatures to
Kimi or other provider targets.

### Reference sources

- `internal/signature/kimi_validation.go`
- `internal/signature/provider_compatibility.go`
- `internal/signature/claude_messages_sanitize.go`

### Implementation details

1. Add Kimi signature inspection, including supported raw transport lengths,
   unpadded standard Base64 checks, bounded length, entropy checks, and foreign
   signature rejection.
2. Extend provider detection and compatibility decisions with Kimi.
3. Ensure Kimi signature detection never reclassifies local Claude, Codex,
   Gemini, xAI, or Antigravity signatures.
4. Update sanitizer behavior to preserve only valid target-compatible blocks.
5. Do not weaken existing strict Claude validation.

### Tests and validation

```sh
go test ./internal/signature
go test ./internal/translator/...
go test ./internal/runtime/executor -run Kimi
```

## Phase 6: Codex Multi-Turn Reasoning Replay

### Goal

Finish the local Codex cache by adding reference-quality runtime use in HTTP,
WebSocket, and streaming paths.

### Reference sources

- `internal/runtime/executor/codex_executor_reasoning.go`
- `internal/runtime/executor/codex_executor_execute.go`
- `internal/runtime/executor/codex_executor_stream.go`
- `internal/runtime/executor/codex_websockets_execute.go`
- `internal/runtime/executor/codex_websockets_stream.go`

### Local targets

- Add `codex_executor_reasoning.go` and focused tests.
- Merge only narrow hooks into Codex HTTP and WebSocket executors.
- Reuse the existing local `codex_reasoning_replay_cache.go`; improve it only
  where a test proves it lacks a needed concurrency or bound guarantee.

### Implementation details

1. Use existing session-key precedence: Claude Code scope, execution metadata,
   prompt cache keys, turn metadata, headers, then safe caller fallback.
2. Restore cached encrypted reasoning at the correct input anchor.
3. Preserve ordering across accumulated turns with explicit turn markers.
4. Match tool outputs to historical tool calls and align shortened call IDs.
5. Never inject duplicate encrypted content or duplicate calls.
6. Persist completed turns from HTTP, SSE, and WebSocket completed events.
7. Clear only stale replay state after an upstream invalid-thinking-signature
   response.
8. Ensure websocket lifecycle errors and retry behavior remain local-compatible.

### Tests and validation

```sh
go test ./internal/cache -run CodexReasoningReplay
go test ./internal/runtime/executor -run Codex.*Replay
go test ./internal/runtime/executor -run CodexWebsocket
```

## Phase 7: API-Key Model Compatibility Metadata

### Goal

Add explicit model mapping metadata needed for compatible provider routing and
context-window capabilities.

### Reference sources

- `internal/config/config_types.go`
- `internal/registry/model_registry.go`
- model config hashing and management config handlers.

### Proposed fields

1. `is-compat` on Claude, Codex/xAI, Gemini, and OpenAI-compatible mappings.
2. `max-context-length` on the same mappings.
3. `alpha-search` per Codex key.

### Implementation details

1. Extend local config structs, clone behavior, normalization, YAML tags, and
   example config.
2. Propagate model metadata through the registry and request metadata so
   executor eligibility checks can read it without reparsing YAML.
3. Include new fields in model/config hash calculations and watcher diffs.
4. Expose fields in management config serialization and validation.
5. Preserve local provider-specific mapping fields, especially NVIDIA and Zen
   options; do not replace their structs with reference versions.

### Tests and validation

```sh
go test ./internal/config ./internal/registry ./internal/watcher
go test ./internal/api/handlers/management
```

## Phase 8: Codex API-Key Alpha Search

### Goal

Allow alpha-search routing through configured Codex API-key credentials and
their compatible base URLs while preserving OAuth behavior.

### Dependencies

Phase 7 must be complete because this requires `alpha-search` configuration and
model capability metadata.

### Implementation details

1. Select credentials with an Alpha Search-specific policy that supports API
   key and OAuth paths safely.
2. Resolve selected API-key base URL and model alias before issuing the request.
3. Retain official OAuth Alpha Search behavior as a fallback.
4. Reuse central status/error mapping rather than leaking raw transport errors.
5. Consider Grok Shell `/v1/models` behavior as a separate subfeature because
   it requires a reference-only `internal/client/grokbuild` package.

### Validation

```sh
go test ./internal/api -run Alpha
go test ./internal/api/handlers/management -run Alpha
go test ./sdk/shinway/auth
```

## Phase 9: Claude OAuth Safety and Credential Identity

### Goal

Port narrowly-scoped auth correctness improvements without changing successful
existing OAuth request behavior.

### Reference sources

- `internal/auth/claude/identity.go`
- `internal/auth/claude/oauth_response.go`
- relevant changes to Claude auth refresh/profile code.

### Implementation details

1. Add guarded access helpers for shared `Auth.Metadata` so concurrent refresh,
   request, and profile operations cannot cause concurrent-map panics.
2. Add per-credential device identity pool generation/normalization.
3. Decode OAuth response encodings consistently: gzip, deflate, Brotli, and
   supported compress modes.
4. Use existing dependencies where available. `brotli` is already in local
   `go.mod`; add dependencies only after license and need review.
5. Audit direct metadata reads/writes in Claude auth and executor paths and
   route them through the new helpers.

### Tests and validation

```sh
go test -race ./internal/auth/claude
go test ./internal/runtime/executor -run Claude.*Auth
```

## Phase 10: Claude Native-Client Compatibility Helpers

### Goal

Port independent Claude compatibility improvements after thinking replay and
credential identity are stable.

### Subfeatures

1. Stable diagnostics/request continuity state.
2. Client and upstream detection helpers.
3. Credential-scoped agent/session identity.
4. Deterministic MCP tool aliases with collision-safe reverse mapping.
5. Better fast-mode error classification.
6. OpenAI-compatible tool-result normalization where required.

### Reference sources

- `internal/runtime/executor/claude_executor_diagnostics.go`
- `internal/runtime/executor/claude_executor_fast_error.go`
- `internal/runtime/executor/helps/claude_diagnostics.go`
- `internal/runtime/executor/helps/claude_client_detection.go`
- `internal/runtime/executor/helps/claude_credential_identity.go`
- `internal/runtime/executor/helps/claude_mcp_alias.go`
- `internal/runtime/executor/helps/claude_upstream.go`
- `internal/runtime/executor/helps/openai_compat_tool_results.go`

### Implementation approach

1. Split into one small, tested subfeature per change set; do not combine all
   helpers in one patch.
2. Integrate diagnostics only after verifying the local header and cloaking
   model. The local Claude executor differs from reference and must retain its
   existing behavior.
3. Add aliasing only if it preserves all client-visible tool names exactly on
   response restoration.
4. Keep fast-mode logic isolated from normal upstream errors.
5. Run Claude executor regression tests after every subfeature.

### Validation

```sh
go test ./internal/runtime/executor -run Claude
go test ./internal/runtime/executor/helps -run Claude
go test -race ./internal/runtime/executor ./internal/runtime/executor/helps
```

## Phase 11: Provider-Neutral Reasoning Summary Intent

### Goal

Preserve user intent for visible reasoning summaries across translation paths
without inventing reasoning effort or breaking local NVIDIA behavior.

### Reference sources

- `internal/thinking/summary.go`
- `internal/thinking/apply.go`
- provider-specific thinking appliers.

### Implementation details

1. Add a canonical summary-intent model with three states: unspecified,
   explicitly enabled, and explicitly disabled.
2. Extract intent from OpenAI Chat/Responses, Claude, Gemini, Antigravity,
   Interactions, Codex, xAI, and Kimi request shapes.
3. Apply intent to target provider payloads only where supported.
4. Do not create reasoning effort merely because a summary field appears.
5. Preserve local `internal/thinking/provider/nvidia` behavior. Add NVIDIA
   tests before merging shared `thinking.ApplyThinking` changes.
6. Retain existing translation hook behavior that skips generic thinking passes
   where the local hook owns conversion.

### Validation

```sh
go test ./internal/thinking/...
go test ./internal/translator/...
go test ./internal/runtime/executor -run 'NVIDIA|Zen|Claude|Codex|XAI|Kimi'
```

## Phase 12: Registry and Catalog Refresh

### Goal

Bring current useful static model definitions from the reference without
removing local model entries.

### Scope

1. Add `grok-imagine-video-1.5` only after confirming endpoint and output
   compatibility with the local xAI executor.
2. Compare each reference model catalog entry to local models by identifier,
   context window, modalities, and aliases.
3. Merge only missing or corrected records.
4. Never replace the entire local `models.json`; it may contain fork-specific
   models and local provider data.

### Tests

```sh
go test ./internal/registry
go test ./internal/runtime/executor -run XAI
```

## Phase 13: SDK and Home Execution Registry Migration

### Status

Do not start without a separate design review and explicit approval.

### Why deferred

The reference `sdk/cliproxy` execution/auth/service architecture is a large
foundational divergence. It owns credential scheduling, Home dispatch/ack,
in-flight reporting, concurrency release, execution registry, subscriber
lifecycle, and provider registration. Local code has independently evolved
runtime, provider, usage, and web management behavior.

### Required design before implementation

1. Document current local ownership of service startup, auth selection, Home
   lifecycle, execution registry, and usage reporting.
2. Map each reference SDK package to local `sdk/shinway` equivalents.
3. Decide whether to:
   - port selected protocol capabilities into the existing SDK, or
   - migrate SDK architecture in a dedicated compatibility branch.
4. Produce API compatibility tables for exported local SDK packages.
5. Add migration and rollback plans for active Home nodes and in-flight work.
6. Run race and integration testing with Home disabled and enabled.

### Explicit exclusions until approved

- Renaming `sdk/shinway` to `sdk/cliproxy`.
- Replacing local auth conductor/scheduler implementations wholesale.
- Replacing local server startup/lifecycle code.
- Replacing local web management or usage analytics services.

## Phase 14: Documentation

### Goal

Add SDK and watcher documentation only after the documented local APIs are
confirmed to exist and are supported.

### Reference sources

- `docs/sdk-usage.md` and Chinese translation.
- `docs/sdk-advanced.md` and Chinese translation.
- `docs/sdk-access.md` and Chinese translation.
- `docs/sdk-watcher.md` and Chinese translation.

### Implementation details

1. Create local `docs/` only when documentation is ready to publish.
2. Rewrite module paths, command names, package names, and examples for
   `shinway`; do not copy reference branding.
3. Verify every code sample against `go test` or a compile-only example.
4. Do not document unimplemented Phase 13 architecture.

## Cross-Cutting Security and Reliability Requirements

1. Never log raw API keys, signed-thinking payloads, session keys, Home KV
   values, credentials, or client content.
2. Hash key components used in Home KV keys.
3. Bound all state stored from upstream or clients: bytes, entries, blocks,
   turns, TTL, scanner buffers, and CAS retries.
4. Treat cache reads as required only when silently omitting replay would cause
   malformed upstream history. Otherwise fail safely by skipping replay and
   recording a non-sensitive warning.
5. Use conditional deletes/replacements whenever concurrent request completion
   could otherwise erase a newer session state.
6. Test cancellation, partial stream consumption, malformed SSE, upstream error
   events, and invalid signatures.
7. Preserve response model aliases and client tool names after any upstream
   normalization or remapping.

## Final Verification Matrix

After all approved phases are complete:

```sh
gofmt -w <changed Go files only>
go test ./internal/cache ./internal/signature ./internal/thinking/...
go test ./internal/translator/...
go test ./internal/runtime/executor/...
go test ./internal/auth/claude
go test ./internal/config ./internal/registry ./internal/watcher
go test ./internal/api/...
go test ./...
git diff --check
git status --short
```

For batches that change shared maps, caches, Home state, credential metadata,
or stream wrappers, additionally run focused race tests:

```sh
go test -race ./internal/cache ./internal/auth/claude ./internal/runtime/executor
```

Manual regression checks:

1. Start the existing local server and management panel using the current local
   Docker/Compose workflow. Confirm it still builds and retains `db/` data.
2. Confirm NVIDIA and Zen configured models continue routing with current
   thinking behavior.
3. Confirm Claude API-key compatible multi-turn tool sessions succeed when the
   client omits old signed thinking blocks.
4. Confirm OAuth Claude, non-compatible mappings, and missing-session requests
   do not activate Claude replay.
5. Confirm Codex, Kimi, xAI, Gemini, Antigravity, OpenAI, and Interactions
   representative stream/non-stream translations preserve model aliases, tool
   identity, and existing error behavior.

## Completion Definition

The port is complete only when every approved phase has:

1. An implementation adapted to the local architecture.
2. Tests for new behavior and regression-sensitive local behavior.
3. Passing focused validation and relevant full-suite validation.
4. A reviewed diff with no unintended deletions or module/branding replacement.
5. Confirmed preservation of local-only functionality and the user’s working
   tree changes.
