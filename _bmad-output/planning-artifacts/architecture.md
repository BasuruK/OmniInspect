---
stepsCompleted: [1, 2, 3, 4, 5, 6, 7, 8]
workflowType: 'architecture'
lastStep: 8
status: 'complete'
completedAt: '2026-04-26'
inputDocuments:
  - _bmad-output/brainstorming/brainstorming-session-2026-04-19-2221.md
  - docs/SUBSCRIBER_ISOLATION_SOLUTION.md
  - docs/ARCHITECTURE_AND_MULTI_SUBSCRIBER_PLAN.md
  - DESIGN.md
project_name: OmniInspect
user_name: Basuruk
date: '2026-04-26'
---

# Architecture Decision Document

_This document builds collaboratively through step-by-step discovery. Sections are appended as we work through each architectural decision together._

## Input Documents

| Document | Purpose |
|----------|---------|
| `brainstorming-session-2026-04-19-2221.md` | Solution decisions, edge cases resolved |
| `SUBSCRIBER_ISOLATION_SOLUTION.md` | Original solution options (IP filtering, explicit param, CLIENT_IDENTIFIER, session binding, hybrid) |
| `ARCHITECTURE_AND_MULTI_SUBSCRIBER_PLAN.md` | Current architecture, obsolete components, multi-subscriber implementation plan |
| `DESIGN.md` | UX design specification (target visual language, layout rules, component standards, interaction rules) |

---

## Session Context

**Topic:** Per-subscriber message isolation for OmniView/OmniInspect trace messages over Oracle AQ

+**Selected Solution:** Dynamic procedure per subscriber (`TRACE_MESSAGE_<name>('msg')`) with Oracle AQ correlation-based routing

+**Key Insight:** Subscriber identity is bound at compile-time via method name, and routing is enforced at the Oracle queue level using `message_properties_.correlation` + subscriber rules (`tab.CORRELATION IS NULL OR tab.CORRELATION = '<name>'`).

---

## Resolved Decisions (from Brainstorming)

| Decision | Value |
|----------|-------|
| Name format | `<30 chars`, `^[A-Za-z_]+$` (letters and underscores only) |
| Name uniqueness | Auto-assigned funny name (e.g., BARNACLE, PICKLES) via FunnyNameGenerator |
| Creation | Idempotent - check if exists, only create if missing |
| SQL injection | Strict format validation before DDL generation |
| Package invalidation | Accepted risk - app redeploys on restart |
| Danger zone options | Per-subscriber method deletion OR drop entire OMNI_TRACER_API package |
| Auto-redeploy | Already implemented - if package missing, OmniView redeploys |
| Permissions | Any database user can call generated procedures |
| Scalability | N subscribers supported, no hard limit |
| Message routing | Correlation-based subscriber rules at Oracle queue level (DEC-6). `Enqueue_Event___` sets `message_properties_.correlation := subscriber_name_`. `Register_Subscriber` adds rule `tab.CORRELATION IS NULL OR tab.CORRELATION = '<name>'`. Broadcast messages (NULL correlation) reach all; subscriber-specific messages reach only the matching subscriber |

---

## Problem Summary

IFS Cloud executes trace calls under IFS app user identity, NOT the debugging OmniView user. No way to correlate:
- **Caller**: IFS Cloud session user (producer)
- **Subscriber**: OmniView user who wants to debug (listener)

**Current state:** All subscribers see all messages (broadcast)

**Desired state:** Each message delivered to exactly the intended subscriber

---

## Project Context Analysis

### Requirements Overview

**Functional Requirements:**
- FR-1: Generate unique `TRACE_MESSAGE_<subscriber_id>()` procedure per subscriber on registration
- FR-2: Idempotent creation - check if procedure exists before creating
- FR-3: **Modify Settings UI** - Add danger zone option to drop subscriber-specific procedure
- (FR-4 REMOVED: Package-drop poses unacceptable risk — accidental deletion affects all subscribers)
- FR-5: Auto-redeploy package on startup if missing
- FR-6: Strict name format validation (`^[A-Za-z_]+$`) to prevent SQL injection
- FR-7: Display the subscriber's method name in TUI - Show the user the exact procedure they must call in their PL/SQL code to receive subscriber-isolated messages (e.g., `OMNI_TRACER_API.TRACE_MESSAGE_SUB_0CC283A4...`)

**Non-Functional Requirements:**
- NFR-1: Package invalidation is acceptable - app recovers on restart
- NFR-2: Any database user can call generated procedures
- NFR-3: Support N concurrent subscribers (no hard limit)
- NFR-4: Developer ergonomics - `TRACE_MESSAGE_xxx('msg')` same friction as `Trace_Message('msg')`

**Scale & Complexity:**
- Primary domain: Real-time TUI + Oracle AQ messaging
- Complexity level: Medium (feature enhancement to existing working system)
- Estimated new/modified components: Settings screen modification (2-3 new danger zone options)
- UI patterns: S key opens Settings (already implemented), webhook address currently saved there

### Technical Constraints & Dependencies
- Go + Bubble Tea v2 + Lip Gloss v2 (existing)
- Oracle 19c with ODPI-C for database connectivity
- Existing `OMNI_TRACER_API` package must be extended
- Existing hexagonal architecture must be respected
- Settings screen already exists at `internal/adapter/ui/database_settings.go`

### Cross-Cutting Concerns Identified
- SQL injection prevention via format validation
- Package invalidation recovery mechanism (already implemented)
- Backward compatibility with existing `Trace_Message()` callers
- Oracle sharded queue limitation: `recipient_list` not supported (ORA-24205) — message routing handled via `message_properties_.correlation` and subscriber rules with `tab.CORRELATION IS NULL OR tab.CORRELATION = '<name>'` (see DEC-6)

---

## Core Architectural Decisions

### Decision Priority Analysis

**Critical Decisions (Block Implementation):**
- Procedure naming convention (DECIDED ✅)
- Procedure generation location (DECIDED ✅)
- Method name display location (DECIDED ✅)
- Cartoon character name list (PENDING)

**Important Decisions (Shape Architecture):**
- None remaining

**Deferred Decisions (Post-MVP):**
- None

---

### DEC-1: Procedure Naming Convention

**Status**: DECIDED ✅

**Decision**: Auto-assign funny cartoon character names from curated list

**Details**:
- Curated list of 100+ cartoon character names (Mickey, Donald, Bugs, Daffy, Scooby, Tom, Jerry, etc.)
- System auto-assigns on subscriber creation
- User has NO visibility into or choice over the name
- Collision handling: system picks another available name automatically

**Resulting procedure name**: `TRACE_MESSAGE_<FUNNY_NAME>` (e.g., `TRACE_MESSAGE_BARNACLE`)

**Display in TUI**: `OMNI_TRACER_API.TRACE_MESSAGE_BARNACLE('msg')`

---

### DEC-2: Procedure Generation Location

**Status**: DECIDED ✅

**Decision**: Always generate procedures inside `OMNI_TRACER_API` package

**Details**:
- Procedures are added to existing package body via ALTER PACKAGE
- NOT standalone schema objects — all methods must be inside Omni_Tracer_API
- Package invalidation is acceptable risk — monitor and adjust if it becomes a problem
- Enables single package to contain all generated methods
- Simplifies permissions and deployment

**Implementation Note**:
- `Enqueue_Event___()` in the base package supports an optional `subscriber_name_` parameter that sets `message_properties_.correlation` for routing
- Generated procedures call `Enqueue_Event___()` internally with `subscriber_name_` set to the assigned funny name
- Message routing is enforced at the Oracle queue level via correlation-based subscriber rules (see DEC-6)

---

### DEC-3: Method Name Display Location

**Status**: DECIDED ✅

**Decision**: Display in Main Screen header with visual indication

**Details**:
- User always sees their method name in the Main Screen header/status area
- Visual indication to draw attention to it
- Shows the exact procedure they must call in their PL/SQL code

---

### DEC-4: Cartoon Character Name List

**Status**: DECIDED ✅

**Decision**: Curated list of ~150 iconic cartoon character names

**Name List**:
```text
Mickey, Donald, Goofy, Pluto, Minnie, Daisy, Chip, Dale, Huey, Dewey, Louie,
Simba, Mufasa, Scar, Nala, Timon, Pumbaa, Zazu,
Bugs, Daffy, Porky, Tweety, Sylvester, Yosemite, Elmer, Foghorn,
Tom, Jerry, Spike, Butch, Tuffy,
Scooby, Shaggy, Fred, Daphne, Velma,
SpongeBob, Patrick, Squidward, Krabs, Sandy, Gary,
Woody, Buzz, Jessie, Rex, Slinky, Hamm, Bo,
Mike, Sulley, Randall, Celia, Roz,
Marlin, Nemo, Dory, Gill, Bloat, Bubbles, Peach,
Stitch, Lilo, Jumba, Pleakley, Gantu,
Aladdin, Jasmine, Genie, Abu, Jafar, Sultan, Iago,
Belle, Beast, Gaston, Lumière, Cogsworth, MrsPotts, Chip,
Peter, Wendy, Michael, Tinker, Nana,
Hercules, Meg, Pegasus, Phil,
Mulan, Mushu, LiShan, ShanYu,
Tarzan, Jane, Clayton,
Ariel, Sebastian, Flounder, Ursula, Triton,
Cinderella, Jaq, Gus, FairyGodmother,
SleepingBeauty, Maleficent, Flora, Fauna, Merryweather,
SnowWhite, EvilQueen, Grumpy, Happy, Bashful, Sneezy, Dopey, Doc,
Pinocchio, Jiminy, Geppetto, BlueFairy,
RobinHood, LittleJohn, MaidMarian, Sheriff,
PeterPan, CaptainHook, TinkerBell,
Alice, MadHatter, CheshireCat, QueenOfHearts,
Winnie, Piglet, Eeyore, Tigger, Rabbit, Owl, Kanga, Roo,
Sonic, Tails, Knuckles, Amy, Shadow, Silver, Blaze,
Mario, Luigi, Peach, Toad, Yoshi, Bowser, Koopa,
Bart, Homer, Marge, Lisa, Maggie, Flanders,
FamilyGuy, Stewie, Brian,
SouthPark, Stan, Kyle, Cartman, Kenny,
Rick, Morty, Summer, Beth, Jerry, Squanchy,
Yoda, Luke, Leia, Han, Vader, Chewbacca,
Batman, Superman, WonderWoman,
Gadget, Monterey, Zipper, Pikachu, Ash, Misty, Brock,
Goku, Vegeta, Piccolo, Gohan, Frieza, Cell,
Naruto, Sasuke, Sakura, Kakashi, Hinata,
Luffy, Zoro, Nami, Usopp, Sanji, Chopper,
Guts, Griffith, Casca, Farnese, Serpico
```

**Validation**: All names match `^[A-Za-z_]+$` (letters and underscores only, no numbers)

---

### DEC-5: Cartoon Name List Storage

**Status**: DECIDED ✅

**Decision**: Store as Go constant array in `internal/core/domain/funny_names.go`

**Rationale**: Domain layer - fits the value object pattern; follows existing project conventions

---

### DEC-6: Message Routing Strategy — Correlation-Based Subscriber Rules

**Status**: DECIDED ✅ (2026-05-09, revised from payload filtering to correlation)

**Problem**: Oracle Sharded Queues (and their successor TxEventQ) do NOT support `recipient_list` on enqueue (`ORA-24205: feature not supported for sharded queues`). The original design assumed `message_properties_.recipient_list` could route messages to specific subscribers at the queue level. This is only supported on classic AQ queues, not sharded/TxEventQ.

**Rejected Alternatives**:

| Alternative | Reason Rejected |
|-------------|-----------------|
| **Switch to classic AQ queue** | Would lose sharded queue performance benefits; breaking migration for existing deployments |
| **Per-subscriber queue** | Requires separate queues per subscriber, C dequeue layer changes, dynamic queue targeting in Go — too much complexity for the value delivered |
| **Application-level payload filtering** | Works but pushes routing to Go — all subscribers dequeue and deserialize all messages, discarding non-matching ones. Unnecessary when Oracle can filter natively via correlation rules |

**Decision**: Correlation-based routing using Oracle AQ subscriber rules

**How it works**:
1. **Enqueue**: `Enqueue_Event___()` sets `message_properties_.correlation := subscriber_name_`. When `subscriber_name_` is NULL (broadcast via `Trace_Message()`), correlation is NULL.
2. **Subscriber Registration**: `Register_Subscriber()` calls `DBMS_AQADM.ADD_SUBSCRIBER` with a rule: `tab.CORRELATION IS NULL OR tab.CORRELATION = '<subscriber_name_>'`
3. **Dequeue**: Each subscriber dequeues with `consumer_name := '<funny_name>'`. Oracle AQ applies the rule and only delivers messages where correlation matches OR is NULL.
4. **Result**:
   - `Trace_Message('hello')` → correlation=NULL → matches `IS NULL` → **all subscribers receive** (broadcast)
   - `TRACE_MESSAGE_BARNACLE('hello')` → correlation='BARNACLE' → matches only BARNACLE's rule → **only BARNACLE receives** (isolated)

**PL/SQL Changes** (`assets/sql/Omni_Tracer.sql`):

`Enqueue_Event___` — Remove `recipient_list`, add correlation:
```sql
-- REMOVED (causes ORA-24205):
--   IF subscriber_name_ IS NOT NULL THEN
--       message_properties_.recipient_list(1) := SYS.AQ$_AGENT(subscriber_name_, NULL, NULL);
--   END IF;

-- ADDED:
message_properties_.correlation := subscriber_name_;
```

`Register_Subscriber` — Add correlation rule:
```sql
DBMS_AQADM.ADD_SUBSCRIBER (
    queue_name => TRACER_QUEUE_NAME,
    subscriber => sub_,
    rule       => 'tab.CORRELATION IS NULL OR tab.CORRELATION = ''' || subscriber_name_ || ''''
);
```

**Go Changes**: NONE. No domain model changes. No service filtering. No C changes. Routing is handled entirely at the Oracle queue level before messages reach Go.

**Subscriber Identity Model**:

| Identifier | Example | Scope | Purpose |
|------------|---------|-------|---------|
| UUID Name (`name`) | `SUB_825418F0_B22F_...` | Go / BoltDB | Internal identity — BoltDB storage key, stable across restarts |
| FunnyName (`funnyName`) | `BARNACLE` | Oracle AQ | Oracle consumer name, correlation value, generated procedure name, subscriber rule |

`ConsumerName()` returns FunnyName when assigned (always after registration), falling back to UUID for legacy subscribers.

Subscriber routing is represented by `message_properties_.correlation`; the JSON payload does not include a `SUBSCRIBER` metadata field.

**Key properties**:
- Routing handled at Oracle queue level — zero Go/C code changes for routing
- Broadcast messages (`Trace_Message()`) reach all subscribers via `correlation IS NULL` rule
- Subscriber-specific messages reach only the matching subscriber via correlation value match
- No dead messages — Oracle only delivers messages matching each subscriber's rule
- Clean queue — no messages accumulate for non-matching subscribers

---

## Implementation Patterns & Consistency Rules

### Existing Patterns (From AGENTS.md & DESIGN.md)

The following patterns are already established and MUST be followed:

| Document | Patterns |
|----------|----------|
| `AGENTS.md` | Constructor `New...`, Interfaces `...er`, Package naming, Error handling, Hexagonal architecture |
| `DESIGN.md` | Bubble Tea v2 lifecycle, Lip Gloss styling, Component standards, Interaction rules |
| `internal/core/domain/` | Entity pattern, Value objects, Sentinel errors |

### New Patterns for Multi-Subscriber Feature

#### Pattern 1: Funny Name Assignment

**File**: `internal/core/domain/funny_names.go`

```go
// FunnyNames returns the curated list of cartoon character names
// for subscriber procedure naming.
func FunnyNames() []string {
    return []string{
        "Mickey", "Donald", "Goofy", // ... (full list)
    }
}

// ValidateName returns true if name is in the funny names list.
// Used for SQL injection prevention - names MUST come from this list.
func IsValidFunnyName(name string) bool {
    // ... lookup in list
}
```

#### Pattern 2: Procedure Ownership Method

**File**: `internal/service/subscribers/` (subscriber_service.go)

```go
// EnsureSubscriberProcedure ensures the TRACE_MESSAGE_<name> procedure is owned
// by the current subscriber and deploys it when needed.
func (s *SubscriberService) EnsureSubscriberProcedure(ctx context.Context, subscriberName string) error

// DropSubscriberProcedure removes the subscriber's procedure from the package.
func (s *SubscriberService) DropSubscriberProcedure(ctx context.Context, subscriberName string) error

// DropAllProcedures drops the entire OMNI_TRACER_API package.
func (s *SubscriberService) DropAllProcedures(ctx context.Context) error
```

#### Pattern 3: Dynamic PL/SQL Generation

**DDL Format** (procedure added to existing package body via ALTER PACKAGE):

```sql
-- Pattern for owned generated procedure within package body:
-- @SECTION: SUBSCRIBER_GENERATED_METHOD : SUB_BARNACLE
PROCEDURE TRACE_MESSAGE_BARNACLE(
    message_   IN VARCHAR2,
    log_level_ IN VARCHAR2 DEFAULT 'INFO'
)
IS
BEGIN
    Enqueue_Event___(
        log_level_        => log_level_,
        payload           => message_,
        subscriber_name_  => 'BARNACLE'
    );
END TRACE_MESSAGE_BARNACLE;
-- @END_SECTION: SUBSCRIBER_GENERATED_METHOD : SUB_BARNACLE
```

**Required Prerequisite**: `Enqueue_Event___()` must support optional `subscriber_name_` routing before generated procedures can call it.
**Ownership Rule**: The generated method block must carry the subscriber ownership markers above. If the same funny-name method exists but is owned by a different subscriber, the app must choose another funny name and deploy a new method.

**Validation Rules**:
- Name MUST be validated against funny name list before DDL generation
- No user-provided names accepted - only system-assigned funny names
- Format check: `^[A-Za-z_]+$` before any DDL execution
- Idempotent: Check the owned procedure block before creating or reusing it

#### Pattern 4: Settings UI Danger Zone

**File**: `internal/adapter/ui/database_settings.go`

Danger zone section in Settings screen:
- Visual distinction (red/warning styling)
- Confirmation required before destructive actions
- ONE option: Drop subscriber-specific procedure only

NOTE: Drop entire OMNI_TRACER_API package (FR-4) has been PARKED due to risk concerns.
If needed in the future, it should be implemented as an admin-only operation with separate access controls.

### Enforcement Guidelines

**All AI Agents MUST**:
- Follow existing patterns from `AGENTS.md` and `DESIGN.md`
- Store funny name list in `internal/core/domain/funny_names.go`
- Use idempotent procedure creation (check if exists before creating)
- Implement proper error handling for package invalidation scenarios
- Follow Lip Gloss styling from DESIGN.md for new UI elements

---

## Starter Template Evaluation

### Primary Technology Domain

**Not Applicable** - This is a brownfield feature enhancement to an existing OmniView/OmniInspect project.

The project already has an established tech stack:
- **Language**: Go
- **UI Framework**: Bubble Tea v2 + Lip Gloss v2
- **Database**: Oracle 19c with ODPI-C
- **Architecture**: Hexagonal (Ports and Adapters)

No starter template required. Technical decisions are already made by the existing architecture.

### Architectural Approach

For this enhancement, we follow the **extend, not replace** principle:
- Existing `OMNI_TRACER_API` package → extend with new procedures
- Existing `SubscriberService` → add procedure generation on registration
- Existing Settings UI → add danger zone options
- Existing `TracerService` → remains unchanged (receiving logic unchanged)

---

## Project Structure & Boundaries

### Complete Project Directory Structure

```text
OmniInspect/
├── AGENTS.md                                    # Agent guidelines & patterns
├── Makefile                                     # Build commands
├── DESIGN.md                                   # UX design specification
├── README.md
├── go.mod
├── go.sum
├── cmd/
│   └── omniview/
│       └── main.go                              # Composition root
│
├── internal/
│   ├── adapter/
│   │   ├── config/                             # Settings loader
│   │   ├── storage/
│   │   │   ├── boltdb/                         # BoltDB implementation
│   │   │   └── oracle/
│   │   │       ├── oracle_adapter.go           # [MODIFY] DDL execution
│   │   │       ├── dequeue_ops.c               # CGO dequeue operations
│   │   │       ├── dequeue_ops.h
│   │   │       ├── queue.go
│   │   │       ├── sql_parse.go
│   │   │       └── subscriptions.go
│   │   └── ui/
│   │       ├── model.go                        # Bubble Tea model
│   │       ├── messages.go                     # Message handlers
│   │       ├── main_screen.go                  # [MODIFY] Display procedure name
│   │       ├── database_settings.go            # [MODIFY] Danger zone options
│   │       ├── welcome.go
│   │       ├── chrome.go
│   │       ├── loading.go
│   │       ├── styles/
│   │       └── animations/
│   │
│   ├── core/
│   │   ├── domain/
│   │   │   ├── errors.go                       # Sentinel errors
│   │   │   ├── subscriber.go                   # Subscriber entity
│   │   │   ├── queue_message.go                 # Queue message entity
│   │   │   ├── config.go                      # Configuration values
│   │   │   ├── permissions.go                  # Permission definitions
│   │   │   ├── webhook.go                      # Webhook configuration
│   │   │   ├── database_settings.go             # Database settings
│   │   │   └── funny_names.go                  # [NEW] Cartoon character names
│   │   │
│   │   └── ports/
│   │       └── repository.go                   # Interface definitions
│   │
│   ├── service/
│   │   ├── subscribers/
│   │   │   └── subscriber_service.go            # [MODIFY] Procedure generation
│   │   ├── tracer/
│   │   │   └── tracer_service.go                # Trace event handling
│   │   ├── permissions/
│   │   ├── webhook/
│   │   └── updater/
│   │
│   ├── app/
│   └── updater/
│
├── docs/
│   ├── ARCHITECTURE_AND_MULTI_SUBSCRIBER_PLAN.md
│   ├── SUBSCRIBER_ISOLATION_SOLUTION.md
│   └── BLOCKING_DEQUEUE_IMPLEMENTATION.md
│
└── _bmad-output/
    ├── planning-artifacts/
    │   └── architecture.md                     # This document
    └── brainstorming/
        └── brainstorming-session-2026-04-19-2221.md
```

### Architectural Boundaries

**Component Boundaries:**
```text
Composition Root (cmd/omniview/main.go)
         │
         ▼
Service Layer
 ┌──────────────────┐  ┌──────────────────┐  ┌──────────────┐
 │SubscriberService│  │ TracerService    │  │ Permissions  │
 │ [MODIFY]        │  │                  │  │              │
 │ - GenerateProc  │  │ - EventListener  │  │              │
 │ - DropProc      │  │ - MessageHandle │  │              │
 │ - DropAll       │  │                  │  │              │
 └──────────────────┘  └──────────────────┘  └──────────────┘
         │
    ┌────┴────────┐
    ▼             ▼
 Ports      Adapters
(Interfaces)  ┌─────────────┐  ┌───────────────┐
              │   Oracle   │  │   BoltDB      │
              │ [MODIFY]  │  │               │
              │ - DDL Exec│  │               │
              └───────────┘  └───────────────┘
              ┌─────────────────────────────────┐
              │         UI (Bubble Tea)          │
              │  ┌────────────┐ ┌────────────┐   │
              │  │MainScreen │ │DB Settings│   │
              │  │[MODIFY]   │ │[MODIFY]   │   │
              │  └────────────┘ └────────────┘   │
              └─────────────────────────────────┘
```

**Domain Layer:**
```text
┌──────────────────────────────────────────────────────────────┐
│                    Domain Layer                              │
│  ┌────────────────┐  ┌────────────────┐  ┌───────────────┐  │
│  │   Subscriber   │  │ QueueMessage  │  │ FunnyNames   │  │
│  │   Entity      │  │   Entity      │  │ [NEW]        │  │
│  └────────────────┘  └────────────────┘  └───────────────┘  │
│  ┌────────────────┐  ┌────────────────┐                    │
│  │    Errors      │  │   Config      │                    │
│  │  Sentinel     │  │  ValueObject  │                    │
│  └────────────────┘  └────────────────┘                    │
└──────────────────────────────────────────────────────────────┘
```

### Requirements to Structure Mapping

| Requirement | Files | Changes |
|-------------|-------|---------|
| **FR-1, FR-2, FR-6**: Generate `TRACE_MESSAGE_<funny>` procedure | `internal/core/domain/funny_names.go` | **NEW** - Curated name list + validation |
| | `internal/service/subscribers/subscriber_service.go` | **MODIFY** - Add `EnsureSubscriberProcedure()` |
| | `internal/adapter/storage/oracle/oracle_adapter.go` | **MODIFY** - Add DDL execution for procedure creation |
| **FR-3**: Danger zone options | `internal/adapter/ui/database_settings.go` | **MODIFY** - Add "Drop my procedure" |
| **FR-5**: Auto-redeploy on startup | `internal/adapter/storage/oracle/oracle_adapter.go` | **VERIFY** - Check existing redeploy logic |
| **FR-7**: Display method name in TUI | `internal/adapter/ui/main_screen.go` | **MODIFY** - Show `OMNI_TRACER_API.TRACE_MESSAGE_<NAME>()` in header |

### Cross-Cutting Concerns Mapping

| Concern | Location |
|---------|----------|
| **SQL Injection Prevention** | `internal/core/domain/funny_names.go` - `IsValidFunnyName()` |
| **Package Invalidation Recovery** | `internal/adapter/storage/oracle/oracle_adapter.go` - existing redeploy |
| **Error Handling** | `internal/core/domain/errors.go` - add sentinel errors for procedure ops |
| **CGO/Oracle Alignment** | `dequeue_ops.c` ↔ `dequeue_ops.h` ↔ `oracle_adapter.go` |

### Integration Points

**Oracle Package (PL/SQL) — External to this codebase:**
```text
OMNI_TRACER_API
├── Trace_Message(message_, log_level_)      # Original - unchanged
├── Enqueue_Event___(..., subscriber_name_) # Internal helper with optional subscriber routing
└── TRACE_MESSAGE_<FUNNY_NAME>(...)        # [NEW] Generated per subscriber
```

**Dynamic Deployment Note:**
Per-subscriber procedures are generated as runtime DDL and executed via `ExecuteStatement()` to add them inside the `OMNI_TRACER_API` package body. This requires ALTER PACKAGE which may invalidate the entire package — this is an accepted risk.

**Data Flow:**
```text
IFS Cloud → OMNI_TRACER_API.TRACE_MESSAGE_<NAME>('msg')
         → Enqueue_Event___(..., subscriber_name_=<NAME>)
         → AQ Queue
         → OmniView dequeues
         → Only matching subscriber receives
```

---

## Architecture Validation Results

### Coherence Validation ✅

**Decision Compatibility:** All decisions work together:
- Funny name format `^[A-Za-z_]+$` aligns with SQL injection prevention via `IsValidFunnyName()`
- Idempotent creation pattern works with `ExecuteStatement()` for DDL
- Package invalidation risk is explicitly accepted — monitor and adjust if needed
- All methods inside `OMNI_TRACER_API` package (no standalone schema objects)

**Pattern Consistency:** Implementation patterns support decisions:
- `funny_names.go` value object pattern matches existing domain layer conventions
- `subscriber_service.go` modification pattern follows established service patterns
- Dynamic DDL via `ExecuteStatement()` enables runtime procedure generation

**Structure Alignment:** Project structure supports architecture:
- `funny_names.go` in `core/domain` as planned
- `subscriber_service.go` modifications in existing location
- Oracle adapter has `ExecuteStatement()` already

---

### Requirements Coverage Validation ✅

| Requirement | Status | Evidence |
|-------------|--------|----------|
| FR-1, FR-2, FR-6: Generate procedure | ✅ | `ExecuteStatement()` at runtime DDL |
| FR-3: Danger zone options (FR-4 PARKED) | ✅ | `database_settings.go` modification |
| FR-5: Auto-redeploy | ✅ | Existing `DeployAndCheck()` |
| FR-7: Display method name | ✅ | `main_screen.go` header area |

**Additional Requirements (User-Confirmed):**
1. `Enqueue_Event___()` must support optional `subscriber_name_` routing as part of this development
2. All generated procedures must be inside `OMNI_TRACER_API` package — no standalone schema objects
3. Package invalidation is accepted risk

---

### Implementation Readiness Validation ✅

**Decision Completeness:** All decisions documented with examples
**Structure Completeness:** Complete directory tree with `[MODIFY]/[NEW]` annotations
**Pattern Completeness:** All major patterns have code examples

---

### Gap Analysis Results

**Critical Gaps:** None remaining after user decisions

**Resolved:**
- `Enqueue_Event___()` will be extended with subscriber routing during implementation
- All procedures inside package (no standalone procedures)
- Package invalidation accepted risk

---

### Architecture Completeness Checklist

**✅ Requirements Analysis**
- [x] Project context thoroughly analyzed
- [x] Scale and complexity assessed
- [x] Technical constraints identified
- [x] Cross-cutting concerns mapped

**✅ Architectural Decisions**
- [x] Critical decisions documented
- [x] Technology stack fully specified
- [x] Integration patterns defined
- [x] Package invalidation risk accepted

**✅ Implementation Patterns**
- [x] Naming conventions established
- [x] Structure patterns defined
- [x] Communication patterns specified
- [x] Process patterns documented

**✅ Project Structure**
- [x] Complete directory structure defined
- [x] Component boundaries established
- [x] Integration points mapped
- [x] Requirements to structure mapping complete

---

### Architecture Readiness Assessment

**Overall Status:** READY FOR IMPLEMENTATION

**Confidence Level:** High — all critical decisions made, patterns documented

**Key Decisions Resolved:**
- Procedure naming: Funny cartoon character names (auto-assigned)
- Generation approach: Inside `OMNI_TRACER_API` package via ALTER PACKAGE
- `Enqueue_Event___()`: Must support optional subscriber routing in the base package
- Package invalidation: Accepted risk

**First Implementation Priority:**
`funny_names.go` (domain value object) → `Enqueue_Event___()` subscriber routing extension → procedure generation in `subscriber_service.go`

---

## Next Step

[C] Continue to complete the architecture workflow
