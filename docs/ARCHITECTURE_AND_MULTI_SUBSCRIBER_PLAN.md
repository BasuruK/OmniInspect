# OmniInspect Architecture Documentation & Multi-Subscriber Implementation Plan

## Document Information

| Field | Value |
|-------|-------|
| **Document Version** | 2.0 |
| **Created** | January 30, 2026 |
| **Last Updated** | January 31, 2026 |
| **Project** | OmniInspect (OmniView) |
| **Branch** | `architecture-change-from-oci-to-blocked-polling` |
| **Purpose** | Architecture cleanup, obsolete code identification, and multi-subscriber implementation planning |

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Current State Analysis](#2-current-state-analysis)
3. [Files to Remove (Obsolete)](#3-files-to-remove-obsolete)
4. [Files to Keep](#4-files-to-keep)
5. [Code Cleanup Actions](#5-code-cleanup-actions)
6. [Multi-Subscriber Implementation Plan](#6-multi-subscriber-implementation-plan)
7. [Alternative Subscriber Identification Strategies](#7-alternative-subscriber-identification-strategies)
8. [Package Invalidation & Production Safety](#8-package-invalidation--production-safety)
9. [Domain-Driven Design & Hexagonal Architecture Review](#9-domain-driven-design--hexagonal-architecture-review)
10. [Bubble Tea TUI Design](#10-bubble-tea-tui-design)
11. [Usability Improvements & Feature Ideas](#11-usability-improvements--feature-ideas)

---

## 1. Executive Summary

### What Changed

The project transitioned from an **OCI Push Notification** model (broken due to firewall/NAT issues) to a **Kafka-Style Blocking Consumer** pattern. This architectural change:

1. **Eliminated the need for OCI Subscriptions** - The `SubscriptionManager` and all related C/Go callback infrastructure is now obsolete
2. **Simplified the event loop** - `TracerService` now uses a simple blocking dequeue loop instead of subscription callbacks + periodic polling
3. **Retained subscriber names for dequeue** - While enqueue broadcasts messages, dequeue still requires a subscriber name for multi-consumer queues

### The Contradiction You Identified

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        THE SUBSCRIBER PARADOX                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ENQUEUE (Trace_Message)           DEQUEUE (BulkDequeue)                    │
│  ─────────────────────────         ────────────────────                     │
│  • Does NOT use subscriber         • REQUIRES subscriber name               │
│  • Broadcasts to ALL consumers     • Each subscriber gets own copy          │
│  • Single method for all users     • Messages filtered by subscriber        │
│                                                                             │
│  PROBLEM: How can different users send messages that only THEIR             │
│           subscriber receives, when Trace_Message doesn't know              │
│           who's calling it?                                                 │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

This document provides a comprehensive plan to solve this problem.

---

## 2. Current State Analysis

### 2.1 Architecture Overview (Post-Blocking Dequeue)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          CURRENT ARCHITECTURE                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  cmd/omniview/main.go                                                       │
│  └── Application Bootstrap                                                  │
│      ├── Initialize BoltDB Adapter                                          │
│      ├── Load Configurations (ConfigLoader)                                 │
│      ├── Initialize Oracle Adapter                                          │
│      ├── Create Services                                                    │
│      │   ├── PermissionService                                              │
│      │   ├── TracerService ◄── Now uses blocking dequeue (WORKING)          │
│      │   └── SubscriberService                                              │
│      └── Start Event Listener                                               │
│                                                                             │
│  internal/service/tracer/tracer_service.go                                  │
│  └── blockingConsumerLoop()                                                 │
│      └── db.BulkDequeueTracerMessages() ◄── Blocking call to Oracle         │
│          └── dequeue_ops.c :: DequeueManyAndExtract()                       │
│              └── PL/SQL: OMNI_TRACER_API.Dequeue_Array_Events()             │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 2.2 Message Flow (Current)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           CURRENT MESSAGE FLOW                               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Database Side                         Application Side                     │
│  ─────────────────                     ──────────────────                   │
│                                                                             │
│  Any PL/SQL Code                       OmniView Client                      │
│       │                                     │                               │
│       │ OMNI_TRACER_API.Trace_Message()     │                               │
│       │ (No subscriber specified)           │                               │
│       ▼                                     │                               │
│  ┌──────────────┐                           │                               │
│  │ OMNI_TRACER_ │                           │                               │
│  │ _QUEUE       │◄─────────────────────────►│ BulkDequeueTracerMessages()   │
│  │ (Sharded/TEQ)│   blocking dequeue        │ with subscriber name          │
│  └──────────────┘                           │                               │
│       │                                     │                               │
│       │ Message visible to ALL              │                               │
│       │ registered subscribers              │                               │
│       ▼                                     ▼                               │
│  Each subscriber gets                  Only messages for THIS               │
│  a COPY of the message                 subscriber are returned              │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 2.3 Obsolete Components

The following components were designed for OCI push subscriptions and are **no longer used**:

| Component | Location | Purpose (Obsolete) | Status |
|-----------|----------|-------------------|--------|
| `SubscriptionManager` | `internal/adapter/subscription/subscription_manager.go` | OCI subscription management | **OBSOLETE** |
| `queue_callback.c` | `internal/adapter/subscription/queue_callback.c` | C callback for OCI notifications | **OBSOLETE** |
| `queue_callback.h` | `internal/adapter/subscription/queue_callback.h` | C header for callbacks | **OBSOLETE** |
| `notification_handler.go` | `internal/adapter/subscription/notification_handler.go` | Go export for C callback | **OBSOLETE** |
| `notifyChan` | `internal/service/tracer/tracer_service.go` | Channel for push notifications | **OBSOLETE** |

---

## 3. Files to Remove (Obsolete)

### 3.1 Complete Removal List

The following files should be **deleted entirely**:

```
internal/adapter/subscription/
├── subscription_manager.go    ◄── DELETE (OCI subscription logic)
├── queue_callback.c           ◄── DELETE (C callback implementation)
├── queue_callback.h           ◄── DELETE (C callback header)
└── notification_handler.go    ◄── DELETE (Go-C bridge for callbacks)
```

### 3.2 Justification for Each File

#### `subscription_manager.go`

**Lines of Code**: ~140 lines  
**Purpose**: Managed OCI subscriptions via `dpiConn_subscribe()`  
**Why Obsolete**: 
- OCI push notifications require the database to connect BACK to your application
- This is blocked by firewalls/NAT in most corporate environments
- The blocking dequeue pattern replaces this entirely
- The `Subscribe()` and `Unsubscribe()` methods are never called in the new architecture

**Dependencies Removed**:
- `runtime/cgo` (cgo.Handle for callback context)
- CGO imports for `dpiSubscr` types

#### `queue_callback.c`

**Lines of Code**: ~115 lines  
**Purpose**: C implementation of `onQueueNotification()` callback and `RegisterOracleSubscription()`  
**Why Obsolete**:
- `onQueueNotification()` was called by Oracle when messages arrived (push model)
- `RegisterOracleSubscription()` set up the subscription with `dpiConn_subscribe()`
- Neither function is called in the blocking dequeue model
- The entire push notification mechanism is replaced

#### `queue_callback.h`

**Lines of Code**: ~30 lines  
**Purpose**: Header declaring `RegisterOracleSubscription()` and `UnregisterOracleSubscription()`  
**Why Obsolete**:
- Header for obsolete C functions
- No other code references these functions after cleanup

#### `notification_handler.go`

**Lines of Code**: ~25 lines  
**Purpose**: Exports `notifyGoChannel()` to C for callbacks  
**Why Obsolete**:
- The `//export notifyGoChannel` function was called from C when Oracle pushed a notification
- With blocking dequeue, notifications are synchronous returns, not async callbacks
- This Go-C bridge is no longer needed

### 3.3 Directory Structure After Removal

```
internal/adapter/
├── config/
│   └── settings_loader.go          ◄── KEEP
├── storage/
│   ├── boltdb/
│   │   └── bolt_adapter.go         ◄── KEEP
│   └── oracle/
│       ├── dequeue_ops.c           ◄── KEEP (blocking dequeue)
│       ├── dequeue_ops.h           ◄── KEEP
│       ├── oracle_adapter.go       ◄── KEEP
│       ├── queue.go                ◄── KEEP
│       ├── sql_parse.go            ◄── KEEP
│       └── subscriptions.go        ◄── REVIEW (see section 5)
└── subscription/                   ◄── DELETE ENTIRE DIRECTORY
```

---

## 4. Files to Keep

### 4.1 Core Files (Essential)

| File | Location | Purpose | Notes |
|------|----------|---------|-------|
| `main.go` | `cmd/omniview/` | Application entry point | Needs minor cleanup |
| `app.go` | `internal/app/` | App struct and server | Keep as-is |
| `oracle_adapter.go` | `internal/adapter/storage/oracle/` | Oracle connection management | Core infrastructure |
| `queue.go` | `internal/adapter/storage/oracle/` | `BulkDequeueTracerMessages()` | The new blocking dequeue |
| `dequeue_ops.c` | `internal/adapter/storage/oracle/` | C dequeue implementation | Core of new architecture |
| `dequeue_ops.h` | `internal/adapter/storage/oracle/` | C header | Required |
| `sql_parse.go` | `internal/adapter/storage/oracle/` | SQL parsing utilities | Keep |
| `bolt_adapter.go` | `internal/adapter/storage/boltdb/` | BoltDB operations | Config storage |
| `settings_loader.go` | `internal/adapter/config/` | Configuration loading | Keep |
| `tracer_service.go` | `internal/service/tracer/` | Event listener | Refactored (working) |
| `subscriber_service.go` | `internal/service/subscribers/` | Subscriber management | **IMPORTANT for multi-subscriber** |
| `permissions_service.go` | `internal/service/permissions/` | Permission checks | Keep |

### 4.2 Domain Layer (Keep All)

```
internal/core/
├── domain/
│   ├── config.go          ◄── KEEP (DatabaseSettings, ClientSettings, etc.)
│   ├── permissions.go     ◄── KEEP (DatabasePermissions)
│   └── queue.go           ◄── KEEP (QueueName, Subscriber, QueueMessage)
└── ports/
    ├── config.go          ◄── KEEP (ConfigLoader interface)
    ├── queue.go           ◄── KEEP (Queue port - if exists)
    └── repository.go      ◄── KEEP (DatabaseRepository, ConfigRepository)
```

### 4.3 Assets (Keep All)

```
assets/
├── embed_files.go         ◄── KEEP
├── ins/
│   └── Omni_Initialize.ins ◄── KEEP
└── sql/
    ├── Omni_Tracer.sql    ◄── KEEP (will be modified for multi-subscriber)
    └── Permission_Checks.sql ◄── KEEP
```

### 4.4 File to Review: `subscriptions.go`

**Location**: `internal/adapter/storage/oracle/subscriptions.go`

This file needs to be examined:

```go
// If this file contains Oracle subscription-related code (OCI callbacks),
// it should be DELETED.
//
// If it contains subscriber registration logic (DBMS_AQADM.ADD_SUBSCRIBER),
// it should be KEPT and possibly moved to a more appropriate location.
```

---

## 5. Code Cleanup Actions

### 5.1 Remove SubscriptionManager from TracerService

**File**: `internal/service/tracer/tracer_service.go`

**Current State** (based on your implementation):

```go
type TracerService struct {
    db              ports.DatabaseRepository
    bolt            ports.ConfigRepository
    subscriptionMgr *subscription.SubscriptionManager  // ◄── REMOVE if still present
    processMu       sync.Mutex
}
```

**Target State**:

```go
type TracerService struct {
    db        ports.DatabaseRepository
    bolt      ports.ConfigRepository
    processMu sync.Mutex
}
```

**Actions**:
1. Remove `subscriptionMgr` field from struct
2. Remove import `"OmniView/internal/adapter/subscription"` 
3. Remove any calls to `subscriptionMgr.Subscribe()` or `subscriptionMgr.Unsubscribe()`
4. Remove `cleanUp()` function if it only handled subscription cleanup
5. Remove `notifyChan` variable if still present

### 5.2 Update Constructor

**Current** (may still have):
```go
func NewTracerService(db ports.DatabaseRepository, bolt ports.ConfigRepository) (*TracerService, error) {
    rawConn := db.GetRawConnection()
    rawCtx := db.GetRawContext()
    if rawConn == nil || rawCtx == nil {
        return nil, fmt.Errorf("...")
    }
    subscriptionMgr := subscription.NewSubscriptionManager(rawConn, rawCtx)
    // ...
}
```

**Target**:
```go
func NewTracerService(db ports.DatabaseRepository, bolt ports.ConfigRepository) (*TracerService, error) {
    return &TracerService{
        db:   db,
        bolt: bolt,
    }, nil
}
```

### 5.3 Review and Update main.go

**File**: `cmd/omniview/main.go`

Check for any remaining references to the subscription package:

```go
// Remove this import if present:
// import "OmniView/internal/adapter/subscription"

// The SubscriptionManager should NOT be created in main.go
```

### 5.4 Update DatabaseRepository Interface (If Needed)

**File**: `internal/core/ports/repository.go`

Current interface should be verified:

```go
type DatabaseRepository interface {
    // ... existing methods ...
    BulkDequeueTracerMessages(subscriber domain.Subscriber) ([]string, [][]byte, int, error)
    GetRawConnection() unsafe.Pointer  // ◄── May no longer be needed after cleanup
    GetRawContext() unsafe.Pointer     // ◄── May no longer be needed after cleanup
}
```

**Decision Points**:
- `GetRawConnection()` and `GetRawContext()` were needed for SubscriptionManager
- If no other code uses them, they can be removed from the interface
- However, keep them if you plan future low-level operations

---

## 6. Multi-Subscriber Implementation Plan

### 6.1 Problem Statement

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    MULTI-SUBSCRIBER CHALLENGE                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  SCENARIO: Multiple OmniView clients connect to the same database           │
│                                                                             │
│  Client A (SUB_AAA)        Client B (SUB_BBB)        Client C (SUB_CCC)     │
│       │                         │                         │                 │
│       │                         │                         │                 │
│       ▼                         ▼                         ▼                 │
│  ┌─────────────────────────────────────────────────────────────────┐        │
│  │                      OMNI_TRACER_QUEUE                          │        │
│  │  (Shared queue - all subscribers get ALL messages)              │        │
│  └─────────────────────────────────────────────────────────────────┘        │
│                              ▲                                              │
│                              │                                              │
│                  OMNI_TRACER_API.Trace_Message()                            │
│                  (Called from ANY PL/SQL code)                              │
│                              │                                              │
│  PROBLEM: Trace_Message() doesn't know which subscriber should              │
│           receive the message!                                              │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 6.2 Solution Architecture: Dynamic Procedure Generation

Your proposed solution is elegant and achievable. Here's the detailed implementation plan:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│              DYNAMIC PROCEDURE GENERATION ARCHITECTURE                       │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  1. Client Registration                                                     │
│  ───────────────────────                                                    │
│  Client A connects → RegisterSubscriber("SUB_AAA")                          │
│                   → Generate & Deploy: TRACE_MESSAGE_SUB_AAA()              │
│                   → Return procedure name to client                         │
│                                                                             │
│  2. Message Tracing                                                         │
│  ──────────────────                                                         │
│  User's PL/SQL code calls: OMNI_TRACER_API.TRACE_MESSAGE_SUB_AAA(...)       │
│                         └► Internally calls: Enqueue_For_Subscriber(        │
│                                               'SUB_AAA', ...)               │
│                                                                             │
│  3. Message Retrieval                                                       │
│  ─────────────────────                                                      │
│  Client A calls: BulkDequeue(subscriber='SUB_AAA')                          │
│               → Gets ONLY messages enqueued via TRACE_MESSAGE_SUB_AAA()     │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 6.3 Detailed Implementation Steps

#### Step 1: Modify OMNI_TRACER_API Package

**File**: `assets/sql/Omni_Tracer.sql`

Add new procedures to the package specification:

```sql
-- @SECTION: PACKAGE_SPECIFICATION

CREATE OR REPLACE PACKAGE OMNI_TRACER_API AS 
    TRACER_QUEUE_NAME CONSTANT VARCHAR2(30) := 'OMNI_TRACER_QUEUE';

    -- Core Methods (unchanged)
    PROCEDURE Initialize;
    PROCEDURE Trace_Message(message_ IN VARCHAR2, log_level_ IN VARCHAR2 DEFAULT 'INFO');
    PROCEDURE Dequeue_Array_Events(
        subscriber_name_ IN  VARCHAR2,
        batch_size_      IN  INTEGER,
        wait_time_       IN  NUMBER DEFAULT DBMS_AQ.NO_WAIT,
        messages_        OUT OMNI_TRACER_PAYLOAD_ARRAY,
        message_ids_     OUT OMNI_TRACER_RAW_ARRAY,
        msg_count_       OUT INTEGER
    );
    
    -- Subscriber Management (unchanged)
    PROCEDURE Register_Subscriber(subscriber_name_ IN VARCHAR2);
    
    -- NEW: Multi-Subscriber Support
    -- Generates a unique tracing procedure for a specific subscriber
    PROCEDURE Generate_Subscriber_Procedure(subscriber_name_ IN VARCHAR2);
    
    -- NEW: Drops the generated procedure when subscriber disconnects
    PROCEDURE Drop_Subscriber_Procedure(subscriber_name_ IN VARCHAR2);
    
    -- NEW: Internal procedure called by generated procedures
    PROCEDURE Enqueue_For_Subscriber(
        subscriber_name_ IN VARCHAR2,
        message_         IN VARCHAR2,
        log_level_       IN VARCHAR2 DEFAULT 'INFO'
    );
    
    -- NEW: Check if a subscriber procedure exists
    FUNCTION Subscriber_Procedure_Exists(subscriber_name_ IN VARCHAR2) RETURN BOOLEAN;

END OMNI_TRACER_API;
/
```

#### Step 2: Implement Package Body

```sql
-- @SECTION: PACKAGE_BODY (additions)

-- NEW: Generate a unique trace procedure for a subscriber
PROCEDURE Generate_Subscriber_Procedure(subscriber_name_ IN VARCHAR2)
IS
    PRAGMA AUTONOMOUS_TRANSACTION;
    v_proc_name VARCHAR2(128);
    v_sql       VARCHAR2(4000);
BEGIN
    -- Validate subscriber name (prevent SQL injection)
    IF NOT REGEXP_LIKE(subscriber_name_, '^SUB_[A-Z0-9_]+$') THEN
        RAISE_APPLICATION_ERROR(-20001, 'Invalid subscriber name format');
    END IF;
    
    v_proc_name := 'TRACE_MESSAGE_' || subscriber_name_;
    
    -- Check if procedure already exists
    DECLARE
        v_count NUMBER;
    BEGIN
        SELECT COUNT(*) INTO v_count 
        FROM user_objects 
        WHERE object_name = v_proc_name 
        AND object_type = 'PROCEDURE';
        
        IF v_count > 0 THEN
            -- Procedure already exists, skip creation
            COMMIT;
            RETURN;
        END IF;
    END;
    
    -- Generate the procedure dynamically
    v_sql := '
    CREATE OR REPLACE PROCEDURE ' || v_proc_name || '(
        message_   IN VARCHAR2,
        log_level_ IN VARCHAR2 DEFAULT ''INFO''
    )
    IS
    BEGIN
        OMNI_TRACER_API.Enqueue_For_Subscriber(
            subscriber_name_ => ''' || subscriber_name_ || ''',
            message_         => message_,
            log_level_       => log_level_
        );
    END;';
    
    EXECUTE IMMEDIATE v_sql;
    
    -- Grant execute to public (or specific roles/users as needed)
    EXECUTE IMMEDIATE 'GRANT EXECUTE ON ' || v_proc_name || ' TO PUBLIC';
    
    COMMIT;
EXCEPTION
    WHEN OTHERS THEN
        ROLLBACK;
        RAISE;
END Generate_Subscriber_Procedure;


-- NEW: Drop a subscriber's trace procedure
PROCEDURE Drop_Subscriber_Procedure(subscriber_name_ IN VARCHAR2)
IS
    PRAGMA AUTONOMOUS_TRANSACTION;
    v_proc_name VARCHAR2(128);
BEGIN
    IF NOT REGEXP_LIKE(subscriber_name_, '^SUB_[A-Z0-9_]+$') THEN
        RAISE_APPLICATION_ERROR(-20001, 'Invalid subscriber name format');
    END IF;
    
    v_proc_name := 'TRACE_MESSAGE_' || subscriber_name_;
    
    BEGIN
        EXECUTE IMMEDIATE 'DROP PROCEDURE ' || v_proc_name;
    EXCEPTION
        WHEN OTHERS THEN
            IF SQLCODE != -4043 THEN -- ORA-04043: object does not exist
                RAISE;
            END IF;
    END;
    
    COMMIT;
END Drop_Subscriber_Procedure;


-- NEW: Enqueue with subscriber-specific recipient
PROCEDURE Enqueue_For_Subscriber(
    subscriber_name_ IN VARCHAR2,
    message_         IN VARCHAR2,
    log_level_       IN VARCHAR2 DEFAULT 'INFO'
)
IS
    message_obj         JSON_OBJECT_T;
    enqueue_options_    DBMS_AQ.ENQUEUE_OPTIONS_T;
    message_properties_ DBMS_AQ.MESSAGE_PROPERTIES_T;
    message_handle_     RAW(16);
    json_payload_       CLOB;
    temp_blob_          BLOB; 
    payload_object_     OMNI_TRACER_PAYLOAD_TYPE;
    calling_process_    VARCHAR2(100);
BEGIN
    -- Get calling process name
    calling_process_ := SYS_CONTEXT('USERENV', 'MODULE');
    IF calling_process_ IS NULL THEN
        calling_process_ := 'ANONYMOUS';
    END IF;

    enqueue_options_.visibility := DBMS_AQ.IMMEDIATE;
    
    -- KEY: Set recipient list to specific subscriber only
    message_properties_.recipient_list := SYS.AQ$_RECIPIENT_LIST_T(
        SYS.AQ$_AGENT(subscriber_name_, NULL, NULL)
    );

    message_obj := JSON_OBJECT_T();
    message_obj.PUT('MESSAGE_ID', TO_CHAR(OMNI_tracer_id_seq.NEXTVAL));
    message_obj.PUT('PROCESS_NAME', calling_process_);
    message_obj.PUT('LOG_LEVEL', log_level_);
    message_obj.PUT('PAYLOAD', message_);
    message_obj.PUT('TIMESTAMP', TO_CHAR(SYSTIMESTAMP, 'YYYY-MM-DD"T"HH24:MI:SS.FF3TZH:TZM'));
    message_obj.PUT('SUBSCRIBER', subscriber_name_);
    
    json_payload_ := message_obj.TO_CLOB();
    temp_blob_ := Clob_To_Blob___(json_payload_);
    payload_object_ := OMNI_TRACER_PAYLOAD_TYPE(temp_blob_);

    DBMS_AQ.ENQUEUE(
        queue_name         => TRACER_QUEUE_NAME,
        enqueue_options    => enqueue_options_,
        message_properties => message_properties_,
        payload            => payload_object_,
        msgid              => message_handle_
    );
    
    IF temp_blob_ IS NOT NULL THEN
        DBMS_LOB.FREETEMPORARY(temp_blob_);
    END IF;
END Enqueue_For_Subscriber;


-- NEW: Check if subscriber procedure exists
FUNCTION Subscriber_Procedure_Exists(subscriber_name_ IN VARCHAR2) RETURN BOOLEAN
IS
    v_count NUMBER;
    v_proc_name VARCHAR2(128);
BEGIN
    v_proc_name := 'TRACE_MESSAGE_' || subscriber_name_;
    
    SELECT COUNT(*) INTO v_count 
    FROM user_objects 
    WHERE object_name = v_proc_name 
    AND object_type = 'PROCEDURE';
    
    RETURN v_count > 0;
END Subscriber_Procedure_Exists;
```

#### Step 3: Update Go SubscriberService

**File**: `internal/service/subscribers/subscriber_service.go`

Add methods for procedure management:

```go
// GenerateSubscriberProcedure creates a unique trace procedure for this subscriber
func (ss *SubscriberService) GenerateSubscriberProcedure(subscriber domain.Subscriber) error {
    query := `BEGIN OMNI_TRACER_API.Generate_Subscriber_Procedure(:subscriberName); END;`
    return ss.db.ExecuteWithParams(query, map[string]interface{}{
        "subscriberName": subscriber.Name,
    })
}

// DropSubscriberProcedure removes the subscriber's trace procedure
func (ss *SubscriberService) DropSubscriberProcedure(subscriber domain.Subscriber) error {
    query := `BEGIN OMNI_TRACER_API.Drop_Subscriber_Procedure(:subscriberName); END;`
    return ss.db.ExecuteWithParams(query, map[string]interface{}{
        "subscriberName": subscriber.Name,
    })
}

// GetTraceProcedureName returns the procedure name for this subscriber
func (ss *SubscriberService) GetTraceProcedureName(subscriber domain.Subscriber) string {
    return fmt.Sprintf("TRACE_MESSAGE_%s", subscriber.Name)
}
```

#### Step 4: Update Application Lifecycle

**File**: `cmd/omniview/main.go`

```go
// During startup, after subscriber registration:
subscriber, err := subscriberService.RegisterSubscriber()
if err != nil {
    log.Fatalf("failed to register subscriber: %v", err)
}

// Generate unique trace procedure for this client
if err := subscriberService.GenerateSubscriberProcedure(subscriber); err != nil {
    log.Fatalf("failed to generate subscriber procedure: %v", err)
}

procName := subscriberService.GetTraceProcedureName(subscriber)
fmt.Printf("Your trace procedure: %s.%s(message, log_level)\n", appConfig.Username, procName)

// ... on shutdown:
defer func() {
    if err := subscriberService.DropSubscriberProcedure(subscriber); err != nil {
        log.Printf("warning: failed to drop subscriber procedure: %v", err)
    }
}()
```

### 6.4 Multi-Subscriber Message Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    MULTI-SUBSCRIBER MESSAGE FLOW                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Client A (SUB_AAA)                    Client B (SUB_BBB)                    │
│       │                                     │                               │
│       │ 1. RegisterSubscriber()             │ 1. RegisterSubscriber()       │
│       │ 2. GenerateSubscriberProcedure()    │ 2. GenerateSubscriberProcedure│
│       │                                     │                               │
│       │ Returns: TRACE_MESSAGE_SUB_AAA      │ Returns: TRACE_MESSAGE_SUB_BBB│
│       │                                     │                               │
│       ▼                                     ▼                               │
│  ┌─────────────┐                      ┌─────────────┐                       │
│  │ User's      │                      │ User's      │                       │
│  │ PL/SQL      │                      │ PL/SQL      │                       │
│  │ calls:      │                      │ calls:      │                       │
│  │ TRACE_MSG_  │                      │ TRACE_MSG_  │                       │
│  │ SUB_AAA()   │                      │ SUB_BBB()   │                       │
│  └──────┬──────┘                      └──────┬──────┘                       │
│         │                                    │                              │
│         │                                    │                              │
│         ▼                                    ▼                              │
│  ┌──────────────────────────────────────────────────────────────────┐       │
│  │                     OMNI_TRACER_QUEUE                             │       │
│  │  ┌──────────────────┐       ┌──────────────────┐                 │       │
│  │  │ Message A1       │       │ Message B1       │                 │       │
│  │  │ Recipient: AAA   │       │ Recipient: BBB   │                 │       │
│  │  └──────────────────┘       └──────────────────┘                 │       │
│  └──────────────────────────────────────────────────────────────────┘       │
│         │                                    │                              │
│         │ Dequeue(SUB_AAA)                   │ Dequeue(SUB_BBB)             │
│         ▼                                    ▼                              │
│  Client A receives                    Client B receives                     │
│  ONLY Message A1                      ONLY Message B1                       │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 6.5 Alternative: Standalone Procedures (Non-Package)

If you want the procedures to be truly independent (not invalidate the package):

```sql
-- Generate as standalone procedure (not inside package)
CREATE OR REPLACE PROCEDURE TRACE_MESSAGE_SUB_AAA(
    message_   IN VARCHAR2,
    log_level_ IN VARCHAR2 DEFAULT 'INFO'
)
IS
BEGIN
    OMNI_TRACER_API.Enqueue_For_Subscriber('SUB_AAA', message_, log_level_);
END;
/
```

**Benefits**:
- Recompiling or modifying `OMNI_TRACER_API` won't invalidate the generated procedures
- Each subscriber's procedure is fully independent
- Can be granted different permissions per procedure

**Trade-offs**:
- More objects in the schema
- Need cleanup mechanism when subscribers disconnect

### 6.6 Cleanup Strategy

When a client disconnects:

```go
// Graceful shutdown
func (app *App) Shutdown(subscriber domain.Subscriber, subscriberService *SubscriberService) {
    // 1. Stop the blocking consumer loop
    cancel()
    
    // 2. Drop the subscriber procedure
    if err := subscriberService.DropSubscriberProcedure(subscriber); err != nil {
        log.Printf("warning: failed to drop procedure: %v", err)
    }
    
    // 3. Optionally: Unregister subscriber from queue
    // (May want to keep subscriber for message recovery)
}
```

### 6.7 Security Considerations

1. **SQL Injection Prevention**: Validate subscriber names with regex before using in DDL
2. **Privilege Escalation**: Generated procedures should only call `Enqueue_For_Subscriber`
3. **Orphan Cleanup**: Implement a cleanup job for abandoned procedures
4. **Audit Trail**: Log procedure creation/deletion

```sql
-- Add audit logging
PROCEDURE Generate_Subscriber_Procedure(subscriber_name_ IN VARCHAR2)
IS
BEGIN
    -- ... procedure generation ...
    
    -- Audit log
    INSERT INTO OMNI_TRACER_AUDIT (
        action_timestamp,
        action_type,
        subscriber_name,
        procedure_name,
        performed_by
    ) VALUES (
        SYSTIMESTAMP,
        'PROCEDURE_CREATED',
        subscriber_name_,
        v_proc_name,
        SYS_CONTEXT('USERENV', 'SESSION_USER')
    );
END;
```

---

## 7. Alternative Subscriber Identification Strategies

### 7.1 The Core Problem Revisited

You want:
- **A single, stable `TRACE_MESSAGE()` procedure** for all users
- **Messages to reach only the intended subscriber's dequeue loop**
- **Zero disruption to production** (no package recompilation during active use)
- **No custom procedure per subscriber**

### 7.2 Oracle AQ Native Solutions (Recommended)

Based on Oracle AQ documentation, here are native mechanisms that support subscriber-specific message routing:

#### Strategy A: Use `recipient_list` at Enqueue Time (RECOMMENDED)

Oracle AQ's `MESSAGE_PROPERTIES_T` type includes a `recipient_list` attribute that **allows specifying exactly which subscribers should receive a message**.

```sql
-- MESSAGE_PROPERTIES_T includes:
-- recipient_list  AQ$_RECIPIENT_LIST_T  -- List of specific recipients

-- AQ$_RECIPIENT_LIST_T is:
-- TYPE SYS.AQ$_RECIPIENT_LIST_T IS TABLE OF SYS.AQ$_AGENT INDEX BY BINARY_INTEGER;
```

**Implementation:**

```sql
-- Modified Trace_Message that accepts optional subscriber target
PROCEDURE Trace_Message (
    message_       IN VARCHAR2,
    log_level_     IN VARCHAR2 DEFAULT 'INFO',
    subscriber_    IN VARCHAR2 DEFAULT NULL  -- NEW: Optional target subscriber
)
IS
    enqueue_options     DBMS_AQ.ENQUEUE_OPTIONS_T;
    message_properties  DBMS_AQ.MESSAGE_PROPERTIES_T;
    message_id          RAW(16);
    message_obj         JSON_OBJECT_T;
    payload_clob        CLOB;
    payload_json        JSON;
    recipients          SYS.AQ$_RECIPIENT_LIST_T;
BEGIN
    -- Build message (existing logic)
    message_obj := JSON_OBJECT_T();
    message_obj.put('MESSAGE_ID', SYS_GUID());
    message_obj.put('PROCESS_NAME', 'TRACE');
    message_obj.put('LOG_LEVEL', log_level_);
    message_obj.put('PAYLOAD', message_);
    message_obj.put('TIMESTAMP', TO_CHAR(SYSTIMESTAMP, 'YYYY-MM-DD"T"HH24:MI:SS.FF3"Z"'));
    
    payload_clob := message_obj.to_clob;
    payload_json := JSON(payload_clob);
    
    -- NEW: If subscriber specified, use recipient_list
    IF subscriber_ IS NOT NULL THEN
        recipients(1) := SYS.AQ$_AGENT(subscriber_, NULL, 0);
        message_properties.recipient_list := recipients;
    END IF;
    -- If subscriber_ IS NULL, message goes to ALL subscribers (broadcast)
    
    DBMS_AQ.ENQUEUE(
        queue_name         => TRACER_QUEUE_NAME,
        enqueue_options    => enqueue_options,
        message_properties => message_properties,
        payload            => payload_json,
        msgid              => message_id
    );
    
    COMMIT;
END Trace_Message;
```

**Usage:**

```sql
-- Broadcast to all subscribers (default behavior)
OMNI_TRACER_API.Trace_Message('Hello everyone', 'INFO');

-- Send to specific subscriber only
OMNI_TRACER_API.Trace_Message('Hello SUB_AAA only', 'INFO', 'SUB_AAA');
```

**Benefits:**
- ✅ Single procedure for all use cases
- ✅ No package recompilation needed
- ✅ Native Oracle AQ feature
- ✅ Backward compatible (NULL subscriber = broadcast)
- ✅ Production safe

#### Strategy B: Use `correlation` ID for Filtering

Oracle AQ supports a `correlation` field in message properties that can be used for message filtering at dequeue time.

```sql
-- At enqueue time
message_properties.correlation := subscriber_;

-- At dequeue time (in your Go code or PL/SQL)
dequeue_options.correlation := 'SUB_AAA';  -- Only get messages with this correlation
```

**Implementation:**

```sql
PROCEDURE Trace_Message (
    message_       IN VARCHAR2,
    log_level_     IN VARCHAR2 DEFAULT 'INFO',
    correlation_   IN VARCHAR2 DEFAULT NULL  -- For subscriber filtering
)
IS
    message_properties  DBMS_AQ.MESSAGE_PROPERTIES_T;
    -- ...
BEGIN
    -- Set correlation for filtering
    IF correlation_ IS NOT NULL THEN
        message_properties.correlation := correlation_;
    END IF;
    
    DBMS_AQ.ENQUEUE(
        -- ...
        message_properties => message_properties,
        -- ...
    );
END;
```

**Dequeue with correlation filter (Go side):**

```go
// In your dequeue options, you'd set correlation
// This requires modifying the C dequeue code to accept correlation filter
```

**Trade-offs:**
- ✅ Simple implementation
- ⚠️ Messages still visible to all subscribers (just filtered at dequeue)
- ⚠️ Requires changes to dequeue code

#### Strategy C: Session Context Automatic Detection

Use Oracle's `SYS_CONTEXT` to automatically detect the calling session and map it to a subscriber.

```sql
-- Create a context for subscriber mapping
CREATE OR REPLACE CONTEXT OMNI_SUBSCRIBER_CTX USING OMNI_TRACER_API;

-- Procedure to set current subscriber (call once at session start)
PROCEDURE Set_Session_Subscriber(subscriber_name_ IN VARCHAR2)
IS
BEGIN
    DBMS_SESSION.SET_CONTEXT('OMNI_SUBSCRIBER_CTX', 'SUBSCRIBER', subscriber_name_);
END;

-- Modified Trace_Message that auto-detects subscriber
PROCEDURE Trace_Message (
    message_   IN VARCHAR2,
    log_level_ IN VARCHAR2 DEFAULT 'INFO'
)
IS
    current_subscriber  VARCHAR2(128);
    recipients          SYS.AQ$_RECIPIENT_LIST_T;
BEGIN
    -- Get subscriber from session context
    current_subscriber := SYS_CONTEXT('OMNI_SUBSCRIBER_CTX', 'SUBSCRIBER');
    
    IF current_subscriber IS NOT NULL THEN
        recipients(1) := SYS.AQ$_AGENT(current_subscriber, NULL, 0);
        message_properties.recipient_list := recipients;
    END IF;
    
    -- ... rest of enqueue
END;
```

**Usage:**

```sql
-- At session start (your OmniView client does this)
OMNI_TRACER_API.Set_Session_Subscriber('SUB_AAA');

-- All subsequent Trace_Message calls automatically target SUB_AAA
OMNI_TRACER_API.Trace_Message('This goes to SUB_AAA');
```

**Benefits:**
- ✅ Completely transparent to users
- ✅ No procedure signature changes
- ✅ Session-based automatic routing

**Trade-offs:**
- ⚠️ Requires context creation (one-time DDL)
- ⚠️ Context must be set per session

### 7.3 Comparison Matrix

| Strategy | Single Method | No DDL per Subscriber | Production Safe | Complexity |
|----------|--------------|----------------------|-----------------|------------|
| **A: recipient_list** | ✅ Yes | ✅ Yes | ✅ Yes | Low |
| **B: correlation** | ✅ Yes | ✅ Yes | ✅ Yes | Low |
| **C: Session Context** | ✅ Yes | ✅ Yes | ✅ Yes | Medium |
| **D: Generated Procedures** | ❌ No | ❌ No | ⚠️ Risky | High |

### 7.4 Recommended Approach: Strategy A with Fallback

```sql
-- Final Recommended Implementation
PROCEDURE Trace_Message (
    message_       IN VARCHAR2,
    log_level_     IN VARCHAR2 DEFAULT 'INFO',
    subscriber_    IN VARCHAR2 DEFAULT NULL  -- Optional: target specific subscriber
)
IS
    enqueue_options     DBMS_AQ.ENQUEUE_OPTIONS_T;
    message_properties  DBMS_AQ.MESSAGE_PROPERTIES_T;
    message_id          RAW(16);
    message_obj         JSON_OBJECT_T;
    payload_clob        CLOB;
    payload_json        JSON;
    recipients          SYS.AQ$_RECIPIENT_LIST_T;
    effective_sub       VARCHAR2(128);
BEGIN
    -- Priority: explicit parameter > session context > broadcast
    effective_sub := COALESCE(
        subscriber_,
        SYS_CONTEXT('OMNI_SUBSCRIBER_CTX', 'SUBSCRIBER')
    );
    
    -- Build JSON message
    message_obj := JSON_OBJECT_T();
    message_obj.put('MESSAGE_ID', SYS_GUID());
    message_obj.put('PROCESS_NAME', Get_Process_Name());
    message_obj.put('LOG_LEVEL', log_level_);
    message_obj.put('PAYLOAD', message_);
    message_obj.put('TIMESTAMP', TO_CHAR(SYSTIMESTAMP, 'YYYY-MM-DD"T"HH24:MI:SS.FF3"Z"'));
    message_obj.put('TARGET_SUBSCRIBER', NVL(effective_sub, 'BROADCAST'));
    
    payload_clob := message_obj.to_clob;
    payload_json := JSON(payload_clob);
    
    -- Set recipient if specific subscriber
    IF effective_sub IS NOT NULL THEN
        recipients(1) := SYS.AQ$_AGENT(effective_sub, NULL, 0);
        message_properties.recipient_list := recipients;
    END IF;
    
    DBMS_AQ.ENQUEUE(
        queue_name         => TRACER_QUEUE_NAME,
        enqueue_options    => enqueue_options,
        message_properties => message_properties,
        payload            => payload_json,
        msgid              => message_id
    );
    
    COMMIT;
EXCEPTION
    WHEN OTHERS THEN
        -- Log error but don't disrupt calling code
        NULL;
END Trace_Message;
```

---

## 8. Package Invalidation & Production Safety

### 8.1 Will Recompiling the Package Invalidate Connected Sessions?

**Answer: YES, but with important nuances.**

When you execute `CREATE OR REPLACE PACKAGE BODY` on Oracle:

1. **The package body is recompiled**
2. **All dependent objects are marked INVALID**
3. **Active sessions using the package will get `ORA-04068` on next call**

```
ORA-04068: existing state of packages has been discarded
```

### 8.2 What Happens to Connected Clients?

| Scenario | Impact | Recovery |
|----------|--------|----------|
| **Session using package variables** | Session state lost, `ORA-04068` error | Must reinitialize package |
| **Session in active PL/SQL call** | Call completes, next call fails | Automatic recompilation |
| **New sessions** | No impact | Uses new package version |
| **Dequeue in blocking wait** | May continue or fail depending on timing | Reconnect |

### 8.3 Mitigation Strategies

#### Strategy 1: Edition-Based Redefinition (EBR) - ENTERPRISE ONLY

Oracle EBR allows multiple versions of packages to coexist:

```sql
-- Create new edition
CREATE EDITION v2;
ALTER SESSION SET EDITION = v2;

-- Modify package in new edition
CREATE OR REPLACE PACKAGE BODY OMNI_TRACER_API AS ...

-- Gradually migrate sessions to new edition
```

**Pros:** Zero downtime upgrades
**Cons:** Requires Enterprise Edition, complex setup

#### Strategy 2: Graceful Reconnect in OmniView (RECOMMENDED)

Implement reconnect logic in your Go code:

```go
func (ts *TracerService) blockingConsumerLoop(ctx context.Context, subscriber *domain.Subscriber) {
    backoff := time.Second
    maxBackoff := time.Minute
    
    for {
        select {
        case <-ctx.Done():
            return
        default:
        }
        
        err := ts.processBatch(subscriber)
        if err != nil {
            // Check if it's a package invalidation error
            if isPackageInvalidError(err) {
                log.Printf("Package invalidated, reconnecting in %v...", backoff)
                
                // Wait with backoff
                select {
                case <-time.After(backoff):
                case <-ctx.Done():
                    return
                }
                
                // Attempt reconnect
                if err := ts.db.Reconnect(); err != nil {
                    log.Printf("Reconnect failed: %v", err)
                    backoff = min(backoff*2, maxBackoff)
                    continue
                }
                
                // Reset backoff on success
                backoff = time.Second
                log.Println("Reconnected successfully")
                continue
            }
            
            log.Printf("Dequeue error: %v", err)
        }
    }
}

func isPackageInvalidError(err error) bool {
    return strings.Contains(err.Error(), "ORA-04068") ||
           strings.Contains(err.Error(), "ORA-04061") ||
           strings.Contains(err.Error(), "ORA-06508")
}
```

#### Strategy 3: Maintenance Window Deployment

For production systems:

```
1. Announce maintenance window
2. Gracefully stop all OmniView clients
3. Deploy package changes
4. Restart OmniView clients
5. End maintenance
```

#### Strategy 4: Blue-Green Package Deployment

Create versioned packages:

```sql
-- Version 1 (current)
CREATE OR REPLACE PACKAGE OMNI_TRACER_API_V1 AS ...

-- Version 2 (new)
CREATE OR REPLACE PACKAGE OMNI_TRACER_API_V2 AS ...

-- Synonym points to active version
CREATE OR REPLACE SYNONYM OMNI_TRACER_API FOR OMNI_TRACER_API_V1;

-- Switch to new version (atomic)
CREATE OR REPLACE SYNONYM OMNI_TRACER_API FOR OMNI_TRACER_API_V2;
```

### 8.4 PRAGMA AUTONOMOUS_TRANSACTION Impact

Using `PRAGMA AUTONOMOUS_TRANSACTION` for generated procedures:

```sql
PROCEDURE Generate_Subscriber_Procedure(...)
IS
    PRAGMA AUTONOMOUS_TRANSACTION;  -- Runs in separate transaction
BEGIN
    EXECUTE IMMEDIATE 'CREATE OR REPLACE PROCEDURE ...';
    COMMIT;  -- Commits only the DDL, not caller's transaction
END;
```

**Does NOT prevent invalidation** - it only isolates the transaction context.

### 8.5 Recommended Production Approach

For your use case (adding optional `subscriber_` parameter to `Trace_Message`):

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    PRODUCTION DEPLOYMENT CHECKLIST                           │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  1. ☐ Implement reconnect logic in OmniView Go code                         │
│  2. ☐ Test package change in non-production environment                     │
│  3. ☐ Coordinate with users for brief disruption (~1-2 seconds)             │
│  4. ☐ Deploy package during low-activity period                             │
│  5. ☐ Monitor for ORA-04068 errors in logs                                  │
│  6. ☐ Verify all clients reconnected successfully                           │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 9. Domain-Driven Design & Hexagonal Architecture Review (Updated)

### 9.1 Current Architecture Assessment (Post-Cleanup)

The subscription adapter directory has been **successfully removed**. Current structure:

```
internal/
├── adapter/
│   ├── config/
│   │   └── settings_loader.go    ✅ CLEAN
│   └── storage/
│       ├── boltdb/
│       │   └── bolt_adapter.go   ✅ CLEAN
│       └── oracle/
│           ├── dequeue_ops.c     ✅ CLEAN
│           ├── dequeue_ops.h     ✅ CLEAN
│           ├── oracle_adapter.go ✅ CLEAN
│           ├── queue.go          ✅ CLEAN
│           ├── sql_parse.go      ✅ CLEAN
│           └── subscriptions.go  ⚠️ REVIEW (may have obsolete code)
├── app/
│   └── app.go                    ✅ CLEAN
├── core/
│   ├── domain/
│   │   ├── config.go             ✅ CLEAN
│   │   ├── permissions.go        ✅ CLEAN
│   │   └── queue.go              ✅ CLEAN
│   └── ports/
│       ├── config.go             ✅ CLEAN
│       ├── queue.go              ✅ CLEAN (if exists)
│       └── repository.go         ✅ CLEAN
└── service/
    ├── permissions/
    │   └── permissions_service.go ✅ CLEAN
    ├── subscribers/
    │   └── subscriber_service.go  ✅ CLEAN
    └── tracer/
        └── tracer_service.go      ✅ CLEAN - No SubscriptionManager references
```

### 9.2 Updated Compliance Checklist

| Principle | Status | Notes |
|-----------|--------|-------|
| **Domain layer has no dependencies** | ✅ PASS | `core/domain` only imports standard library |
| **Ports define interfaces** | ✅ PASS | `DatabaseRepository`, `ConfigRepository` well-defined |
| **Adapters implement ports** | ✅ PASS | `OracleAdapter`, `BoltAdapter` implement interfaces |
| **No CGO in ports** | ✅ PASS | `GetRawConnection`/`GetRawContext` removed |
| **Business logic in services** | ✅ PASS | `TracerService` handles event loop logic |
| **Dependency injection** | ✅ PASS | All services use constructor injection |
| **Service single responsibility** | ✅ PASS | Each service has clear responsibility |

### 9.3 Updated Architecture Score

| Aspect | Score | Change | Notes |
|--------|-------|--------|-------|
| **Domain Isolation** | 9/10 | +1 | Obsolete code removed |
| **Port Definition** | 9/10 | — | Clear interfaces |
| **Adapter Implementation** | 9/10 | +1 | Subscription adapter removed |
| **Dependency Direction** | 9/10 | — | Dependencies point inward |
| **Service Orchestration** | 8/10 | — | Well-structured services |
| **Testability** | 8/10 | +1 | Cleaner without CGO callbacks |

**Overall: 8.7/10** (improved from 8.2/10)

### 9.4 Remaining Improvements

1. **Extract Message Processing**:
```go
// tracer/message_processor.go
type MessageProcessor struct {
    handlers map[string]MessageHandler
}

func (mp *MessageProcessor) Process(msg domain.QueueMessage) error {
    handler, ok := mp.handlers[msg.LogLevel]
    if !ok {
        handler = mp.handlers["default"]
    }
    return handler.Handle(msg)
}
```

2. **Add Domain Events for TUI**:
```go
// core/domain/events.go
type MessageReceivedEvent struct {
    Message   QueueMessage
    Timestamp time.Time
}

type ConnectionStateEvent struct {
    Connected bool
    Error     error
}
```

3. **Consider CQRS for Complex Queries**:
```go
// application/queries/
type GetRecentMessagesQuery struct {
    Limit int
    Since time.Time
}

type GetRecentMessagesHandler struct {
    repo ports.MessageRepository
}
```

---

## 10. Bubble Tea TUI Design

### 10.1 Why Bubble Tea?

Based on the [official documentation](https://pkg.go.dev/github.com/charmbracelet/bubbletea):

- **Elm Architecture**: Predictable state management with `Model`, `Update`, `View`
- **Commands for I/O**: Clean separation of side effects via `tea.Cmd`
- **Rich Component Library**: [Bubbles](https://github.com/charmbracelet/bubbles) provides text inputs, tables, spinners, viewports
- **Production Ready**: Used by GitHub CLI, Terraform, AWS, and more

### 10.2 Screen 1: Login Screen

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│                        ╔═══════════════════════════╗                        │
│                        ║    🔮 OmniInspect v1.0   ║                        │
│                        ╚═══════════════════════════╝                        │
│                                                                             │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │  DATABASE CONNECTION                                                │   │
│   ├─────────────────────────────────────────────────────────────────────┤   │
│   │                                                                     │   │
│   │    Host:     │ localhost                                       │   │   │
│   │    Port:     │ 1521                                            │   │   │
│   │    Service:  │ XEPDB1                                          │   │   │
│   │    Username: │ omni_user                                       │   │   │
│   │    Password: │ ••••••••                                        │   │   │
│   │                                                                     │   │
│   │    [ ] Save as default connection                                   │   │
│   │                                                                     │   │
│   │    ┌────────────┐  ┌────────────┐  ┌────────────────┐             │   │
│   │    │  Connect   │  │   Test     │  │ Load Saved ▼  │             │   │
│   │    └────────────┘  └────────────┘  └────────────────┘             │   │
│   │                                                                     │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│   Status: Ready to connect                                                  │
│   ─────────────────────────────────────────────────────────────────────     │
│   Press Tab to navigate • Enter to select • Ctrl+C to quit                  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Bubble Tea Model:**

```go
type LoginModel struct {
    // Form fields (using bubbles/textinput)
    hostInput     textinput.Model
    portInput     textinput.Model
    serviceInput  textinput.Model
    usernameInput textinput.Model
    passwordInput textinput.Model
    
    // UI state
    focusIndex    int
    saveDefault   bool
    status        string
    isConnecting  bool
    err           error
    
    // Saved connections (from BoltDB)
    savedConnections []domain.DatabaseSettings
    selectedSaved    int
}

func (m LoginModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "tab", "shift+tab":
            m.focusIndex = (m.focusIndex + 1) % 6
        case "enter":
            if m.focusIndex == 5 { // Connect button
                return m, m.attemptConnection()
            }
        }
    case ConnectionSuccessMsg:
        // Transition to Mission Control
        return NewMissionControlModel(msg.Connection), nil
    case ConnectionErrorMsg:
        m.err = msg.Error
        m.status = "Connection failed: " + msg.Error.Error()
    }
    return m, nil
}
```

### 10.3 Screen 2: Database Explorer

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  OmniInspect │ Database Explorer                          ┌─ Active ─────┐  │
│  ────────────┴───────────────────────────────────────────│ 🟢 PROD_DB   │  │
│                                                          └──────────────┘  │
│  ┌─ Connections ───────────────────────────────────────────────────────┐   │
│  │                                                                     │   │
│  │  🟢 PROD_DB          prod.oracle.company.com:1521/PRODPDB           │   │
│  │  ⚪ DEV_DB           dev.oracle.local:1521/DEVPDB                   │   │
│  │  ⚪ STAGING_DB       staging.oracle.local:1521/STAGEPDB             │   │
│  │  🔴 TEST_DB          test.oracle.local:1521/TESTPDB    [Offline]    │   │
│  │                                                                     │   │
│  │  ─────────────────────────────────────────────────────────────────  │   │
│  │  + Add New Connection                                               │   │
│  │                                                                     │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌─ Quick Actions ─────────────────────────────────────────────────────┐   │
│  │                                                                     │   │
│  │  [c] Connect    [d] Disconnect    [e] Edit    [x] Delete           │   │
│  │  [r] Refresh    [s] Set Default   [m] Mission Control              │   │
│  │                                                                     │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ───────────────────────────────────────────────────────────────────────   │
│  ↑/↓ Navigate • Enter Select • c Connect • m Mission Control • ? Help      │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Bubble Tea Model:**

```go
type DatabaseExplorerModel struct {
    connections []ConnectionItem
    cursor      int
    selected    int  // -1 if none selected
    
    // Status
    activeDB    string
    width       int
    height      int
    
    // Sub-components
    list        list.Model  // from bubbles
    help        help.Model
}

type ConnectionItem struct {
    Name        string
    Host        string
    Port        int
    Service     string
    Status      ConnectionStatus  // Connected, Disconnected, Error
    IsDefault   bool
}

type ConnectionStatus int

const (
    StatusDisconnected ConnectionStatus = iota
    StatusConnecting
    StatusConnected
    StatusError
)
```

### 10.4 Screen 3: Mission Control

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  OmniInspect │ Mission Control                           🟢 PROD_DB        │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─ Status Bar ────────────────────────────────────────────────────────┐   │
│  │ Subscriber: SUB_ABC123 │ State: ⏳ Blocking │ Health: ████████░░ 80% │   │
│  │ Messages/sec: 42       │ Queue Depth: 156   │ Uptime: 2h 34m 12s     │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌─ Live Trace Messages ───────────────────────────────────────────────┐   │
│  │                                                                     │   │
│  │ 14:32:01.234 │ INFO  │ ORDER_PROCESSOR │ Processing order #12345   │   │
│  │ 14:32:01.456 │ DEBUG │ PAYMENT_SERVICE │ Validating payment...     │   │
│  │ 14:32:01.789 │ INFO  │ PAYMENT_SERVICE │ Payment approved          │   │
│  │ 14:32:02.012 │ WARN  │ INVENTORY_SVC   │ Low stock: SKU-9876       │   │
│  │ 14:32:02.234 │ ERROR │ SHIPPING_SVC    │ Carrier API timeout       │   │
│  │ 14:32:02.567 │ INFO  │ ORDER_PROCESSOR │ Order #12345 completed    │   │
│  │ 14:32:02.890 │ DEBUG │ AUDIT_LOGGER    │ Audit record created      │   │
│  │ ▌                                                     [Auto-scroll] │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌─ Filters ──────────────────────────────┐ ┌─ Stats ─────────────────┐   │
│  │ Level: [ALL ▼]  Process: [ALL ▼]       │ │ INFO:  1,234 │ WARN: 56 │   │
│  │ Search: │________________________│    │ │ DEBUG:   892 │ ERR:  12 │   │
│  └────────────────────────────────────────┘ └─────────────────────────┘   │
│                                                                             │
│  ───────────────────────────────────────────────────────────────────────   │
│  ↑/↓ Scroll • / Search • f Filter • p Pause • c Clear • d DB Explorer      │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Bubble Tea Model:**

```go
type MissionControlModel struct {
    // Connection info
    subscriber    domain.Subscriber
    dbConnection  *oracle.OracleAdapter
    
    // Message stream
    messages      []domain.QueueMessage
    viewport      viewport.Model  // from bubbles - for scrolling
    autoScroll    bool
    
    // Status metrics
    status        StatusMetrics
    
    // Filters
    levelFilter   string  // "ALL", "INFO", "DEBUG", "WARN", "ERROR"
    processFilter string
    searchTerm    string
    searchInput   textinput.Model
    
    // UI state
    width         int
    height        int
    isPaused      bool
    
    // Stats
    messageStats  map[string]int  // count by log level
    
    // Ticker for stats update
    ticker        tea.Cmd
}

type StatusMetrics struct {
    ConnectionState   ConnectionState  // Blocking, Polling, Disconnected
    ConnectionHealth  int              // 0-100
    MessagesPerSecond float64
    QueueDepth        int
    Uptime            time.Duration
    LastMessageTime   time.Time
}

type ConnectionState int

const (
    StateDisconnected ConnectionState = iota
    StateConnecting
    StateBlocking      // Waiting for messages (healthy)
    StatePolling       // Actively receiving
    StateTimeout       // Dequeue timed out
    StateError
)

// Commands for async operations
func listenForMessages(sub domain.Subscriber) tea.Cmd {
    return func() tea.Msg {
        // This runs in background goroutine
        // When message received, send NewMessageMsg
        return NewMessageMsg{Message: msg}
    }
}

func tickStats() tea.Cmd {
    return tea.Tick(time.Second, func(t time.Time) tea.Msg {
        return StatsTickMsg{Time: t}
    })
}
```

### 10.5 Key Bubble Tea Patterns

Based on [Bubble Tea tutorials](https://github.com/charmbracelet/bubbletea/tree/main/tutorials/commands):

**Pattern 1: Commands for I/O**
```go
// Commands are functions that return messages
func connectToDatabase(settings domain.DatabaseSettings) tea.Cmd {
    return func() tea.Msg {
        adapter := oracle.NewOracleAdapter(&settings)
        if err := adapter.Connect(); err != nil {
            return ConnectionErrorMsg{Error: err}
        }
        return ConnectionSuccessMsg{Adapter: adapter}
    }
}
```

**Pattern 2: Subscriptions for Streams**
```go
// For continuous message stream
func (m MissionControlModel) Init() tea.Cmd {
    return tea.Batch(
        m.startMessageStream(),
        tickStats(),
    )
}

func (m *MissionControlModel) startMessageStream() tea.Cmd {
    return func() tea.Msg {
        // Subscribe to message channel
        msg := <-m.messageChannel
        return NewMessageMsg{Message: msg}
    }
}
```

**Pattern 3: Screen Transitions**
```go
func (m LoginModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case ConnectionSuccessMsg:
        // Transition to new screen by returning different model
        return NewMissionControlModel(msg.Adapter, msg.Subscriber), nil
    }
    return m, nil
}
```

### 10.6 Recommended Bubbles Components

From [Bubbles library](https://github.com/charmbracelet/bubbles):

| Component | Use Case |
|-----------|----------|
| `textinput` | Login form fields |
| `viewport` | Scrollable message log |
| `table` | Database list, message table |
| `list` | Connection selector |
| `spinner` | Connection/loading states |
| `progress` | Health/capacity indicators |
| `help` | Keyboard shortcut display |
| `key` | Keybinding management |

---

## 11. Usability Improvements & Feature Ideas

### 11.1 High-Priority Improvements

| Feature | Description | Benefit |
|---------|-------------|---------|
| **Message Filtering** | Filter by log level, process name, time range | Focus on relevant messages |
| **Export to File** | Save messages to JSON/CSV | Post-analysis, sharing |
| **Message Search** | Full-text search with regex | Quick debugging |
| **Bookmarks** | Pin important messages | Track key events |
| **Notifications** | Desktop alerts for ERROR level | Immediate awareness |

### 11.2 Advanced Features

| Feature | Description | Complexity |
|---------|-------------|------------|
| **Multi-Database View** | Side-by-side message streams | High |
| **Message Correlation** | Link related messages by ID | Medium |
| **Custom Dashboards** | User-defined metric panels | High |
| **Alerting Rules** | Pattern-based alerts | Medium |
| **Message Replay** | Re-process historical messages | Medium |
| **Team Sharing** | Share subscriber sessions | High |

### 11.3 Developer Experience

| Feature | Description |
|---------|-------------|
| **SQL Syntax Highlight** | Highlight SQL in messages |
| **JSON Pretty Print** | Format JSON payloads |
| **Stack Trace Folding** | Collapse/expand stack traces |
| **Copy to Clipboard** | One-click message copy |
| **Quick PL/SQL** | Execute trace commands from TUI |

### 11.4 Operational Features

| Feature | Description |
|---------|-------------|
| **Health Dashboard** | Visual connection status |
| **Queue Metrics** | Depth, throughput graphs |
| **Subscriber Management** | Create/delete from TUI |
| **Retention Policy** | Auto-cleanup old messages |
| **Audit Log** | Track who traced what |

---

## Appendix A: File Removal Commands

```bash
# Subscription adapter already removed ✅
# Verify with:
ls internal/adapter/subscription/  # Should fail - directory doesn't exist
```

## Appendix B: Quick Reference

| Type | Location | Example |
|------|----------|---------|
| Entities | `core/domain/` | `Subscriber`, `QueueMessage` |
| Value Objects | `core/domain/` | `QueueConfig`, `DatabaseSettings` |
| Ports (Interfaces) | `core/ports/` | `DatabaseRepository`, `ConfigRepository` |
| Adapters | `adapter/` | `OracleAdapter`, `BoltAdapter` |
| Services | `service/` | `TracerService`, `SubscriberService` |
| Application Entry | `cmd/` | `main.go` |
| SQL Assets | `assets/sql/` | `Omni_Tracer.sql` |

## Appendix C: Oracle AQ recipient_list Reference

From [Oracle AQ Types Documentation](https://docs.oracle.com/en/database/oracle/oracle-database/23/arpls/advanced-queuing-AQ-types.html):

```sql
-- AQ$_RECIPIENT_LIST_T definition
TYPE SYS.AQ$_RECIPIENT_LIST_T IS TABLE OF SYS.AQ$_AGENT INDEX BY BINARY_INTEGER;

-- AQ$_AGENT definition
TYPE SYS.AQ$_AGENT IS OBJECT (
   name       VARCHAR2(512),   -- Subscriber name
   address    VARCHAR2(1024),  -- Queue address (NULL for local)
   protocol   NUMBER           -- Always 0 for local
);

-- Usage in MESSAGE_PROPERTIES_T
-- recipient_list: Only valid for multi-consumer queues
-- If NULL, message goes to all subscribers
-- If set, message goes only to listed agents
```

---

## Document Changelog

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2026-01-30 | Initial comprehensive document |
| 2.0 | 2026-01-31 | Added: Alternative subscriber strategies, Package invalidation analysis, Updated DDD review, Bubble Tea TUI designs, Usability improvements |
