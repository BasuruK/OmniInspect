# OmniInspect Refactoring Guide: DDD & Clean Architecture

This guide provides a step-by-step path to refactor `OmniInspect` from its current monolithic state to a modular, testable **Domain-Driven Design (DDD)** architecture.

We will move code from "Layered" packages (`config`, `database`, `utils`) to "Hexagonal" layers (`core`, `adapter`, `service`).

---

## Target Directory Structure

```text
OmniInspect/
├── cmd/
│   └── omniview/
│       └── main.go            # Entry point (Main)
├── internal/
│   ├── core/                  # PURE DOMAIN LOGIC (No external imports)
│   │   ├── domain/            # Structs/Models (e.g., PermissionStatus)
│   │   └── ports/             # Interfaces (e.g., DatabaseRepository, ConfigLoader)
│   ├── service/               # APPLICATION BUSINESS LOGIC
│   │   ├── permissions/       # Permission checking logic
│   │   └── tracer/            # Tracer logic
│   ├── adapter/               # INFRASTRUCTURE (Implements Ports)
│   │   ├── config/            # File-based config loading
│   │   ├── storage/
│   │   │   ├── oracle/        # ODPI-C implementation (was internal/database)
│   │   │   └── boltdb/        # BoltDB implementation
│   └── app/                   # Application wiring/Bootstrap
├── assets/
│   └── sql/                   # SQL scripts (was internal/sql)
├── third_party/
│   └── odpi/                  # C libraries (was internal/lib/odpi)
├── configs/                   # Default config files (config.cfg, settings.json)
├── go.mod
└── Makefile
```

---

## Phase 1: The Domain Layer (The "Heart")

**DDD Concept:** The **Domain** represents the problem you are solving. It contains **Entities** (data with identity) and **Value Objects** (data without identity). It has **ZERO dependencies** on database drivers, JSON tags, or HTTP frameworks.

### Step 1.1: Define Configuration Domain
**Source:** `internal/config/configs.go`
**Destination:** `internal/core/domain/config.go`

Move the struct definitions here. Strip them of logic.

```go
package domain

// Value Object: Represents database connection settings
type DatabaseSettings struct {
    Database string
    Host     string
    Port     int
    Username string
    Password string
    Default  bool
}

// Value Object: Represents client settings
type ClientSettings struct {
    EnableUtf8 bool
}

// Entity: The full configuration aggregate
type AppConfig struct {
    Database DatabaseSettings
    Client   ClientSettings
}
```

### Step 1.2: Define Permission Domain
**Source:** `internal/config/configs.go` (SystemConfigurations)
**Destination:** `internal/core/domain/permissions.go`

```go
package domain

// Value Object: Result of a permission check
type PermissionStatus struct {
    CanCreateSequence      bool
    CanCreateTable         bool
    CanCreateProcedure     bool
    HasAQAdministratorRole bool
    // ... other fields
    AllPermissionsValid    bool
}
```

---

## Phase 2: The Ports Layer (The "Contracts")

**DDD Concept:** **Ports** (Interfaces) define *what* the application needs from the outside world (Infrastructure). This applies the **Dependency Inversion Principle**: High-level modules (Service) should not depend on low-level modules (Oracle/BoltDB); both should depend on abstractions (Ports).

### Step 2.1: Define Database Port
**Source:** Derived from `internal/database/database.go`
**Destination:** `internal/core/ports/repository.go`

Look at what methods you actually use in `database.go` (`Fetch`, `ExecuteStatement`, `PackageExists`).

```go
package ports

// Port: Defines how we interact with ANY SQL database (Oracle, Mock, etc.)
type DatabaseRepository interface {
    Connect() error
    Close() error
    Fetch(query string) ([]string, error)
    Execute(query string) error
    PackageExists(packageName string) (bool, error)
    DeployPackage(sequences, specs, bodies []string) error
}
```

### Step 2.2: Define Config Port
**Destination:** `internal/core/ports/config.go`

```go
package ports

import "OmniView/internal/core/domain"

// Port: Defines how we load configuration
type ConfigLoader interface {
    Load() (*domain.AppConfig, error)
    Save(config *domain.AppConfig) error
}
```

---

## Phase 3: The Adapter Layer (The "Infrastructure")

**DDD Concept:** **Adapters** implement the Ports. This is where the "dirty" code lives (SQL drivers, File I/O).

### Step 3.1: Oracle Adapter (Refactoring `internal/database`)
**Source:** `internal/database/database.go`
**Destination:** `internal/adapter/storage/oracle/adapter.go`

**CRITICAL CHANGE:** Remove the global `var dbConn *Database`. Instead, use a struct that implements `ports.DatabaseRepository`.

```go
package oracle

import (
    "OmniView/internal/core/domain"
    "OmniView/internal/core/ports"
    "database/sql" // or "C" for ODPI
)

// Adapter: Implements ports.DatabaseRepository
type OracleAdapter struct {
    conn *C.dpiConn // Keep your existing C struct here
    config domain.DatabaseSettings
}

// Constructor: Injects configuration
func NewOracleAdapter(cfg domain.DatabaseSettings) *OracleAdapter {
    return &OracleAdapter{
        config: cfg,
    }
}

// Implementation of Fetch (moved from database.go)
func (o *OracleAdapter) Fetch(query string) ([]string, error) {
    // Use o.conn instead of global dbConn
    // ... existing logic from database.Fetch ...
}

// ... Implement other methods ...
```

### Step 3.2: File Config Adapter (Refactoring `internal/config`)
**Source:** `internal/config/configs.go`
**Destination:** `internal/adapter/config/file_loader.go`

```go
package config

import (
    "OmniView/internal/core/domain"
    "encoding/json"
    "os"
)

type FileConfigLoader struct {
    path string
}

func NewFileLoader(path string) *FileConfigLoader {
    return &FileConfigLoader{path: path}
}

func (l *FileConfigLoader) Load() (*domain.AppConfig, error) {
    // ... existing logic from loadClientConfigurations ...
    // Return domain.AppConfig instead of internal struct
}
```

---

## Phase 4: The Service Layer (The "Business Logic")

**DDD Concept:** **Application Services** orchestrate the flow of data. They retrieve domain objects from repositories, execute domain logic, and save state. They **inject** the Ports.

### Step 4.1: Permission Service
**Source:** `internal/permissions/permissions.go`
**Destination:** `internal/service/permissions/service.go`

**CRITICAL CHANGE:** Instead of importing `internal/database`, we inject `ports.DatabaseRepository`.

```go
package permissions

import (
    "OmniView/internal/core/ports"
    "OmniView/internal/core/domain"
)

type Service struct {
    db ports.DatabaseRepository
}

// Constructor: Dependency Injection!
func NewService(db ports.DatabaseRepository) *Service {
    return &Service{db: db}
}

// Logic moved from HandleDatabasePermissions
func (s *Service) CheckAndDeploy() (bool, error) {
    // Use s.db instead of database.PackageExists
    exists, err := s.db.PackageExists("TXEVENTQ_PERMISSION_CHECK_API")
    if err != nil {
        return false, err
    }
    
    if !exists {
        // Logic to read file and s.db.DeployPackage(...)
    }
    return true, nil
}
```

---

## Phase 5: The Composition Root (Wiring It All)

**DDD Concept:** The **Composition Root** is the *only* place where concrete implementations (Adapters) are assigned to interfaces (Ports).

### Step 5.1: Main Entry Point
**Source:** `main.go`
**Destination:** `cmd/omniview/main.go`

```go
package main

import (
    "OmniView/internal/adapter/config"
    "OmniView/internal/adapter/storage/oracle"
    "OmniView/internal/service/permissions"
    "OmniView/internal/app"
    "log"
)

func main() {
    // 1. Infrastructure (Adapters)
    // Load Config
    cfgLoader := config.NewFileLoader("settings.json")
    appConfig, err := cfgLoader.Load()
    if err != nil {
        log.Fatal(err)
    }

    // Initialize DB Adapter (Inject Config)
    dbAdapter := oracle.NewOracleAdapter(appConfig.Database)
    if err := dbAdapter.Connect(); err != nil {
        log.Fatal(err)
    }
    defer dbAdapter.Close()

    // 2. Services (Inject Adapters)
    permService := permissions.NewService(dbAdapter)

    // 3. Application Bootstrap
    // Run startup checks using the service
    if _, err := permService.CheckAndDeploy(); err != nil {
        log.Printf("Permission check failed: %v", err)
    }

    // 4. Start App
    application := app.New(appConfig, permService)
    application.Start()
}
```

---

## Summary of Moved Code

| Old File | New Location | Reason |
| :--- | :--- | :--- |
| `internal/config/configs.go` (Structs) | `internal/core/domain/config.go` | Pure data, no logic. |
| `internal/config/configs.go` (Load) | `internal/adapter/config/file_loader.go` | File I/O is infrastructure. |
| `internal/database/database.go` | `internal/adapter/storage/oracle/adapter.go` | DB Driver is infrastructure. |
| `internal/permissions/permissions.go` | `internal/service/permissions/service.go` | Business logic orchestration. |
| `internal/utils/utils.go` | `internal/app/bootstrap.go` | Startup scripts belong to App layer. |
| `main.go` | `cmd/omniview/main.go` | Standard Go project layout. |

## Next Steps
1.  Create the folder structure.
2.  Copy the Domain structs first.
3.  Define the Ports interfaces.
4.  Move the Database code and make it implement the Port.
5.  Move the Service code and inject the Port.
6.  Wire it up in `main.go`.
