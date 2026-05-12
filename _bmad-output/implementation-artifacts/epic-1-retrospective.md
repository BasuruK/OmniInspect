---
epic: "Epic 1: Multi-Subscriber Procedure Generation"
date: "2026-05-11"
status: "complete"
---

# Epic 1 Retrospective

## Team

- **Basuru** (Project Lead)
- **Amelia** (Developer)
- **Alice** (Product Owner)
- **Charlie** (Senior Dev)
- **Dana** (QA Engineer)
- **Elena** (Junior Dev)

---

## Part 1: Epic Review

### Delivery Summary

| Story | Title | Status |
|-------|-------|--------|
| 1-1 | Funny Name Value Object | ✅ Done |
| 1-2 | Procedure Generation with Subscriber-Routed Enqueue | ✅ Done |
| 1-3 | SQL Injection Prevention | ✅ Done |
| 1-4 | Auto-Deploy Package on Startup | ✅ Done |
| 1-5 | Correlation-Based Message Routing | ✅ Done |

**Completion:** 5/5 stories (100%)
**Epic Status:** Complete

### What Went Well

1. **AI Code Review Value** — CodeRabbitAI and Kodus flagged: mismatches between documented sources and code, dead methods, inconsistencies between implemented methods and their functionality, missing tests, missed edge cases, error handling gaps. Caught production-impacting bugs including ORA-24205 recipient_list issue.

2. **Multi-source Verification** — Claude Opus said sharded queues can't do named subscriber routing; Gemini said they can. Basuru trusted the suspicion and verified via multiple sources, leading to DEC-6 (correlation-based routing). This saved the entire architecture.

3. **Story 1-5 Emergence** — Not originally planned; discovered during implementation that sharded queues CAN do correlation routing. DEC-6 added late but was critical to making multi-subscriber isolation work.

4. **Refactoring win in Story 1-2** — Consolidated `Enqueue_For_Subscriber()` into the unified `Enqueue_Event___()` private helper, eliminating ~90 lines of duplicate PL/SQL code.

5. **Auto-redeploy mechanism** — SHA256 hash optimization in `deployTracerPackage()` prevents unnecessary redeployment. Idempotent `EnsureSubscriberProcedure()` prevents duplicate procedure creation.

### Challenges & Lessons

1. **AI Review Scope Creep** — CodeRabbitAI drove 100+ PR review comments, causing accidental implementation of remaining Epic 1 stories. The PR grew to 2500+ lines and took 5 days to review. Manual testing burden was significant.

   **Lesson:** Scope bounding for AI reviews is required. Use PR template to document in-scope vs out-of-scope before requesting review. Periodic mid-PR scope checks to catch drift.

2. **AI Confidence vs. Verification** — Claude Opus was confidently wrong about sharded queue limitations. Single AI assertions on architecture-critical questions must be verified.

   **Lesson:** Always verify significant technical claims via multiple AI sources AND direct documentation (Oracle docs, web search). Don't trust a single confident AI assertion on blockers.

3. **Manual Testing Bottleneck** — 2500+ line PR required manual verification of all functionality. No automated E2E tests to lean on.

   **Lesson:** Prioritize automated test coverage for features that touch critical paths (dequeue, routing, procedure generation).

4. **Late Story 1-5 Addition** — Correlation routing wasn't in the original plan. Found during implementation when the assumed approach (recipient_list) failed at runtime.

   **Lesson:** Architecture decisions that depend on platform capabilities (Oracle AQ) need explicit verification before design is finalized.

### Technical Debt Incurred

- Partial subscriber state on registration failure (addressed in Story 1-2 review)
- `Enqueue_For_Subscriber()` duplicate helper (refactored out in Story 1-2)
- Package invalidation risk acknowledged but acceptable (redeploy on restart)

### Key Decisions

| Decision | Value |
|-----------|-------|
| Name format | `<30 chars`, `^[A-Za-z_]+$` |
| Name assignment | Auto-assigned funny name from curated list (collision handling) |
| Enqueue helper | Single `Enqueue_Event___()` with optional `subscriber_name_` routing |
| Message routing | `message_properties_.correlation` + subscriber rules (`tab.CORRELATION IS NULL OR tab.CORRELATION = '<name>'`) |
| Package invalidation | Accepted risk — app redeploys on restart |
| Danger zone options | Per-subscriber method deletion OR drop entire package |

### Breakthrough Moments

1. **DEC-6 (Correlation Routing):** Gemini correctly identified that Oracle AQ correlation + subscriber rules work with sharded/TxEventQ queues despite Claude Opus's confident denial. Changed implementation from payload filtering to Oracle-level routing in a single line.

2. **Story 1-2 Refactor:** Basuru identified `Enqueue_For_Subscriber()` as unnecessary duplication. Consolidated into `Enqueue_Event___()`, reducing ~90 lines of duplicate PL/SQL.

---

## Part 2: Next Epic Preparation

### Epic 2: TUI Procedure Name Display

**Story 2-1: Display Procedure Name in Header**
- Display `OMNI_TRACER_API.TRACE_MESSAGE_BARNACLE('msg')` in main screen header
- Visually prominent (different color, bold)
- **Status:** Ready for dev
- **Dependency:** Epic 1 routing (Story 1-5) confirmed working

### Preparation Tasks

| Task | Owner | Priority |
|------|-------|----------|
| Verify Epic 1 routing is solid before 2-1 starts | Basuru | High |
| Document in-scope/out-of-scope in PR template before requesting AI review | Basuru | High |

---

## Action Items

### Process Improvements

1. **Scope bounding for AI reviews** — Document in PR template what is in-scope vs out-of-scope before requesting AI code review. Mid-PR scope checks to catch drift.
   **Owner:** Basuru
   **Timeline:** Before Epic 2 PR

2. **Verify before implementing** — For significant technical decisions, check Oracle docs directly (web search if Context7 doesn't cover it) and consult multiple AI sources before committing to implementation. Don't trust a single AI's confident assertion on architecture-critical questions.
   **Owner:** All agents
   **Timeline:** Ongoing

3. **Prioritize automated test coverage** — For critical paths (dequeue, routing, procedure generation), ensure automated tests exist before closing stories.
   **Owner:** Amelia
   **Timeline:** Before Epic 3

### Technical Debt

| Item | Priority | Notes |
|------|----------|-------|
| Package invalidation monitoring | Low | Accepted risk; app redeploys on restart |

---

## Sign-off

All team members acknowledge this retrospective accurately captures Epic 1 execution.
