---
project_name: 'OmniInspect'
user_name: 'Basuru'
date: '2026-04-15'
sections_completed: ['technology_stack', 'language_specific_rules', 'framework_specific_rules', 'testing_rules', 'code_quality_style_rules', 'development_workflow_rules', 'critical_dont_miss_rules']
status: 'complete'
rule_count: 57
optimized_for_llm: true
existing_patterns_found: 8
---

# Project Context for AI Agents

_This file contains critical rules and patterns that AI agents must follow when implementing code in this project. Focus on unobvious details that agents might otherwise miss._

---

## Technology Stack & Versions

- Language: Go 1.24.2 with toolchain 1.24.4
- TUI framework: charm.land/bubbletea/v2 v2.0.1
- Styling: charm.land/lipgloss/v2 v2.0.0
- UI components: charm.land/bubbles/v2 v2.0.0
- Local persistence: go.etcd.io/bbolt v1.4.3
- Database integration: Oracle AQ via ODPI-C and CGO
- Supporting languages: PL/SQL for queue package, C for dequeue bindings
- Build workflow: Makefile-driven; use `make build`, `make run`, `make test`, `make fmt`, `make lint`
- Version constraint: use Bubble Tea v2 / Lip Gloss v2 APIs only; do not mix old charmbracelet module paths

## Critical Implementation Rules

### Language-Specific Rules

- Follow existing Go constructor patterns: `New...` functions validate inputs and usually return pointers.
- Keep domain entities validated and infrastructure-agnostic; prefer private struct fields with read-only methods instead of exposing mutable state.
- Use sentinel domain errors from `internal/core/domain/errors.go` and wrap with context via `fmt.Errorf("operation: %w", err)`. Domain sentinel errors (like `ErrInvalidFunnyName`, `ErrNoAvailableNames`) live in `errors.go` alongside other domain errors — not in feature-specific files.
- Prefer pointer receivers for services, adapters, and Bubble Tea models that own mutable state.
- Inject dependencies through constructor arguments or option structs; do not instantiate cross-layer dependencies inside adapters or services.
- Keep nil checks explicit at construction boundaries for required collaborators.
- Preserve package naming and file naming conventions already used in the repo: lowercase package names, underscore-separated Go files where needed.

### Framework-Specific Rules

- Treat `DESIGN.md` as the UI authority ahead of current implementation details; update code to match the design spec when they conflict.
- Keep the Bubble Tea root model as the owner of global UI state, with screen-specific sub-state grouped on the root model.
- Route async work through typed `tea.Msg` messages and `tea.Cmd`; do not perform blocking work inside `View()`.
- Keep screen behavior split into screen-specific `updateX` and `viewX` helpers instead of expanding a monolithic root update/view flow.
- Use `tea.KeyPressMsg` for keyboard handling and handle size-driven layout changes centrally rather than scattering layout state updates.
- Keep `View()` pure and derived from current model state; compose rendered sections with Lip Gloss layout helpers rather than mutating state during rendering.
- Centralize style tokens and reusable panel/text styles in `internal/adapter/ui/styles/styles.go`; avoid ad hoc per-screen color literals unless introducing a deliberate design token.
- Prefer Lip Gloss composition primitives already aligned with the design spec: `JoinVertical`, `JoinHorizontal`, `Place`, framed panels, and explicit width/height calculations.
- Reuse existing form and screen patterns where they already match the new UI architecture, for example the onboarding flow delegating to `AddDatabaseForm` and saving through commands.

### Testing Rules

- Prefer focused package-level tests beside the code under test, using the standard `testing` package and `t.Parallel()` where isolation allows it.
- Test behavior and invariants directly rather than over-mocking; this repo already favors small stubs for ports and real logic around them.
- Keep UI tests deterministic by driving Bubble Tea update flows with explicit typed messages and keypress helpers instead of relying on interactive runtime behavior.
- Cover boundary conditions and regression cases explicitly, especially layout math, wrapping behavior, storage key migration, and lifecycle/shutdown semantics.
- For repository tests, prefer temporary Bolt-backed adapters and verify persisted read/write behavior instead of testing implementation details indirectly.
- For service tests, stub only the required ports and validate observable effects such as cancellation, queue draining, and dispatcher lifecycle.
- When adding a bug fix, add or update the nearest regression test in the same package rather than creating broad end-to-end coverage for a local change.
- Use `make test` as the default validation path; package-targeted `go test` is appropriate for narrow iteration.

### Code Quality & Style Rules

- Keep section-divider comments in the existing repo style when touching files that already use them.
- Prefer short structural comments over explanatory noise; add comments only when they clarify non-obvious behavior or phase boundaries.
- Keep interfaces in `internal/core/ports` and concrete implementations in adapters; do not let interface definitions drift into service or adapter packages.
- Preserve the composition-root pattern in `cmd/omniview/main.go`: dependency wiring belongs there, not in lower layers.
- Follow established naming conventions: `New...` constructors, `...er` interfaces, PascalCase types, lowercase package names, and underscore-separated file names where already used.
- Prefer explicit, narrow interfaces and typed messages over generic maps, stringly typed signaling, or cross-layer shortcuts.
- Keep imports consistent with the current module paths and avoid mixing alternate package sources for the same library family.
- When touching files with existing style tokens, helper structs, or option structs, extend those patterns instead of introducing parallel conventions.

### Development Workflow Rules

- Use the Makefile from the repo root for normal development tasks: `make build`, `make run`, `make test`, `make fmt`, and `make lint`.
- Do not use `go run cmd/omniview/main.go`; this project depends on Makefile-managed CGO flags and Oracle client linker paths.
- Treat Oracle Instant Client and ODPI-C setup as an environment prerequisite, not something to re-encode in application logic.
- When changing Oracle integration or packaging behavior, use the existing setup assets and docs such as `scripts/setup_odpi.py`, the Makefile, and `README.md` as the operational source of truth.
- Keep first-run and persisted configuration behavior compatible with BoltDB-backed local state; changes to onboarding or settings should account for existing persisted data.
- Validate changes with the narrowest useful test/build command during iteration, then fall back to the standard repo commands for final verification.
- Preserve the current repo’s phase ordering: initialize infrastructure first, then compose services, then start the TUI with dependencies already wired.
- Avoid introducing alternate run paths, ad hoc setup flows, or duplicate operational instructions when the repo already defines the canonical workflow.

### Critical Don't-Miss Rules

- Do not break persisted state compatibility. BoltDB contains first-run state, database settings, and webhook configuration, and the adapter already includes legacy migration behavior that must remain safe.
- When changing onboarding or settings flows, keep persisted config hydration and default-selection behavior intact across restarts.
- Do not change webhook-trigger semantics casually: queue messages interpret the Oracle JSON field `send_to_webhook` with `"TRUE"` semantics, and downstream delivery depends on that exact contract.
- Keep webhook validation layered correctly: domain validation enforces basic URL shape, while the webhook service enforces SSRF-related host and network restrictions.
- Avoid bypassing the bounded webhook dispatcher or introducing synchronous webhook delivery from hot message-processing paths.
- Preserve shutdown and cancellation behavior for tracer listeners and the global webhook dispatcher; connection-scoped listener shutdown is intentionally separate from process-wide webhook shutdown.
- Be careful with storage key formats and migrations. Database config keys already support legacy prefixes and escaped storage keys, so new persistence changes must not assume only one historical format exists.
- When touching user-controlled log rendering or webhook payload handling, preserve existing sanitization and validation boundaries rather than weakening them for convenience.

---

## Usage Guidelines

**For AI Agents:**

- Read this file before implementing any code.
- Follow all rules exactly as documented.
- When in doubt, prefer the more restrictive option.
- Update this file if new project-specific patterns emerge.

**For Humans:**

- Keep this file lean and focused on agent needs.
- Update it when the technology stack or operating constraints change.
- Remove rules that become obvious, duplicated, or outdated.

Last Updated: 2026-04-15
