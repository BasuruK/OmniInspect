# DDD Refactoring Report

**Date:** 2026-03-03
**Project:** OmniView / BG_Tracer

---

## Executive Summary

This report documents the DDD (Domain-Driven Design) refactoring changes made to align the project with the best practices outlined in `DDD_REFACTORING_GUIDE.md`.

**Status:** ~90% Complete

---

## Changes Made

### 1. Bug Fixes

#### 1.1 QueueMessage JSON Unmarshal Error
**File:** `internal/core/domain/queue_message.go`

**Issue:** The timestamp field in the JSON was expecting `int64` but Oracle was sending a string.

**Fix:** Updated `queueMessageJSON` to use `json.RawMessage` to handle both formats:
- Unix timestamps (int64)
- String timestamps (Oracle format like `"2026-03-03 13:43:07"`)

#### 1.2 SaveDatabaseConfig Pointer Parameter
**Files:**
- `internal/adapter/storage/boltdb/bolt_adapter.go`
- `internal/core/ports/repository.go`
- `internal/adapter/config/settings_loader.go`

**Issue:** The method was taking a value instead of a pointer, preventing proper JSON marshaling of private fields.

**Fix:** Changed signature from `SaveDatabaseConfig(config domain.DatabaseSettings)` to `SaveDatabaseConfig(config *domain.DatabaseSettings)`

#### 1.3 QueueMessage Nil Pointer in TracerService
**File:** `internal/service/tracer/tracer_service.go`

**Issue:** `json.Unmarshal` was called on a nil pointer.

**Fix:** Initialize the pointer before unmarshaling:
```go
// Before
var msg *domain.QueueMessage
json.Unmarshal(data, msg)

// After
msg := &domain.QueueMessage{}
json.Unmarshal(data, msg)
```

#### 1.4 GetDefaultDatabaseConfig Pointer Fix
**File:** `internal/adapter/storage/boltdb/bolt_adapter.go`

**Issue:** Using value type instead of pointer prevented proper population of private fields.

**Fix:** Changed to use pointer and fixed return statement.

---

### 2. Repository Implementation (DDD Gap Fix)

#### 2.1 New SubscriberRepository
**File:** `internal/adapter/storage/boltdb/subscriber_repository.go`

Created a dedicated repository implementing `ports.SubscriberRepository`:
- `Save(ctx, subscriber)` - Store subscriber
- `GetByName(ctx, name)` - Retrieve by name
- `List(ctx)` - List all subscribers
- `Exists(ctx, name)` - Check existence
- `Delete(ctx, name)` - Remove subscriber

#### 2.2 New PermissionsRepository
**File:** `internal/adapter/storage/boltdb/permissions_repository.go`

Created a dedicated repository implementing `ports.PermissionsRepository`:
- `Save(ctx, permissions)` - Store permissions
- `Get(ctx, schema)` - Retrieve by schema
- `Exists(ctx, schema)` - Check existence

#### 2.3 Updated Repository Interface
**File:** `internal/core/ports/repository.go`

Added `List()` method to `SubscriberRepository` interface.

#### 2.4 New Buckets Added
**File:** `internal/adapter/storage/boltdb/bolt_adapter.go`

Added initialization for new buckets:
- `SubscriberBucket` ("Subscribers")
- `PermissionsBucket` ("Permissions")

---

### 3. Service Layer Refactoring

#### 3.1 SubscriberService
**File:** `internal/service/subscribers/subscriber_service.go`

**Changes:**
- Now uses `ports.SubscriberRepository` instead of `ports.ConfigRepository`
- Added `context.Context` to all methods
- Uses domain factory for creating subscribers

**Before:**
```go
func NewSubscriberService(db ports.DatabaseRepository, bolt ports.ConfigRepository)
func (ss *SubscriberService) RegisterSubscriber() (*domain.Subscriber, error)
```

**After:**
```go
func NewSubscriberService(db ports.DatabaseRepository, subRepo ports.SubscriberRepository)
func (ss *SubscriberService) RegisterSubscriber(ctx context.Context) (*domain.Subscriber, error)
```

#### 3.2 PermissionService
**File:** `internal/service/permissions/permissions_service.go`

**Changes:**
- Now uses `ports.PermissionsRepository` for permission storage
- Still uses `ports.ConfigRepository` for first-run status (needed for deployment workflow)
- Added `context.Context` to `DeployAndCheck`

**Before:**
```go
func NewPermissionService(db ports.DatabaseRepository, bolt ports.ConfigRepository)
func (ps *PermissionService) DeployAndCheck(schema string) (bool, error)
```

**After:**
```go
func NewPermissionService(db ports.DatabaseRepository, permsRepo ports.PermissionsRepository, config ports.ConfigRepository)
func (ps *PermissionService) DeployAndCheck(ctx context.Context, schema string) (bool, error)
```

---

### 4. Application Entry Point Updates

**File:** `cmd/omniview/main.go`

Updated to create and inject the new repositories:

```go
// Create DDD Repositories
subscriberRepo := boltdb.NewSubscriberRepository(boltAdapter)
permissionsRepo := boltdb.NewPermissionsRepository(boltAdapter)

// Services now receive repositories
permissionService := permissions.NewPermissionService(dbAdapter, permissionsRepo, boltAdapter)
subscriberService := subscribers.NewSubscriberService(dbAdapter, subscriberRepo)

// Method calls now include context
permissionService.DeployAndCheck(context.Background(), appConfig.Username())
subscriberService.RegisterSubscriber(context.Background())
```

---

## DDD Readiness Evaluation

### Completed

| Component | Status |
|-----------|--------|
| Rich Domain Entities | ✅ Complete |
| Value Objects | ✅ Complete |
| Repository Interfaces | ✅ Complete |
| Dedicated Repository Implementations | ✅ Complete |
| Service Layer Refactoring | ✅ Complete |
| context.Context Usage | ✅ Complete |

### Remaining Items

| Item | Priority | Notes |
|------|----------|-------|
| TracerService repository refactor | Low | Could use SubscriberRepository |
| Error consolidation | Low | Errors currently in separate domain files |

---

## Benefits Achieved

1. **Separation of Concerns** - Each repository has a single responsibility
2. **Testability** - Can mock repositories for unit testing
3. **Type Safety** - Proper use of pointers and context
4. **Consistency** - All services follow the same pattern
5. **Maintainability** - Clear boundaries between layers

---

## Files Modified

| File | Action |
|------|--------|
| `internal/core/domain/queue_message.go` | Modified |
| `internal/adapter/storage/boltdb/bolt_adapter.go` | Modified |
| `internal/adapter/storage/boltdb/subscriber_repository.go` | Created |
| `internal/adapter/storage/boltdb/permissions_repository.go` | Created |
| `internal/core/ports/repository.go` | Modified |
| `internal/service/subscribers/subscriber_service.go` | Modified |
| `internal/service/permissions/permissions_service.go` | Modified |
| `internal/service/tracer/tracer_service.go` | Modified |
| `internal/adapter/config/settings_loader.go` | Modified |
| `cmd/omniview/main.go` | Modified |

---

*Generated: 2026-03-03*
