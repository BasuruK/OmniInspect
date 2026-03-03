# OmniView

<p align="center">
  <img src="resources/omniview.png" alt="OmniView Logo" width="1000">
</p>

<p align="center">
  <img src="https://img.shields.io/badge/License-Proprietary-red" alt="License">
  <a href="https://github.com/BasuruK/OmniInspect/actions/workflows/go.yml">
    <img src="https://img.shields.io/github/actions/workflow/status/BasuruK/OmniInspect/go.yml" alt="Build">
  </a>
  <a href="https://github.com/BasuruK/OmniInspect">
    <img src="https://img.shields.io/github/languages/top/BasuruK/OmniInspect" alt="Top Language">
  </a>
  <a href="https://github.com/BasuruK/OmniInspect">
    <img src="https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go" alt="Go">
  </a>
  <a href="https://github.com/BasuruK/OmniInspect">
    <img src="https://img.shields.io/badge/PL/SQL-FF0000?style=flat&logo=oracle" alt="PL/SQL">
  </a>
  <a href="https://github.com/BasuruK/OmniInspect">
    <img src="https://img.shields.io/badge/C-A8B9CC?style=flat&logo=c" alt="C">
  </a>
    <a href="https://github.com/BasuruK/OmniInspect">
    <img src="https://img.shields.io/github/stars/BasuruK/OmniInspect" alt="Stars">
  </a>
</p>

## Project Description

OmniView (also known as OmniInspect) is a Message Passing TUI (Text User Interface) application that enables sending event messages and debug traces from Oracle Database. It provides real-time tracing capabilities through Oracle Advanced Queuing (AQ) with a blocking consumer pattern for reliable message delivery.

The application consists of two main components:

1. **Go Desktop Application** - A TUI application that connects to Oracle Database and listens for trace messages
2. **PL/SQL Package (OMNI_TRACER_API)** - Oracle database objects that handle message enqueueing and dequeuing

## Key Functionality

The primary and only essential function for end users is:

```sql
OMNI_TRACER_API.Trace_Message(
    message_    IN CLOB,
    log_level_  IN VARCHAR2 DEFAULT 'INFO'
);
```

This procedure sends a trace message to an Oracle AQ sharded queue. The OmniView application listens for these messages and displays them in real-time.

**Parameters:**
- `message_` - The trace message content (CLOB)
- `log_level_` - Log level (e.g., 'INFO', 'WARN', 'ERROR', 'DEBUG')

**Example Usage:**
```sql
-- Send a simple trace message
BEGIN
    OMNI_TRACER_API.Trace_Message('Processing started', 'INFO');
END;

-- Send a JSON payload
BEGIN
    OMNI_TRACER_API.Trace_Message(
        '{"order_id": 12345, "status": "completed"}',
        'INFO'
    );
END;
```

## Project Structure

```
OmniInspect/
├── cmd/omniview/              # Main application entry point
│   └── main.go                 # Application bootstrap and initialization
├── internal/
│   ├── app/                    # Application core
│   │   └── app.go             # App version and server management
│   ├── adapter/
│   │   ├── config/            # Configuration loading
│   │   │   └── settings_loader.go
│   │   └── storage/
│   │       ├── boltdb/        # BoltDB local storage adapter
│   │       │   ├── bolt_adapter.go
│   │       │   ├── database_settings_repository.go
│   │       │   ├── permissions_repository.go
│   │       │   └── subscriber_repository.go
│   │       └── oracle/        # Oracle database adapter
│   │           ├── oracle_adapter.go
│   │           ├── queue.go
│   │           ├── subscriptions.go
│   │           ├── sql_parse.go
│   │           ├── dequeue_ops.c      # CGO bindings for dequeuing
│   │           └── dequeue_ops.h
│   ├── core/
│   │   ├── domain/            # Domain entities
│   │   │   ├── config.go
│   │   │   ├── database_settings.go
│   │   │   ├── errors.go
│   │   │   ├── permissions.go
│   │   │   ├── queue_message.go
│   │   │   └── subscriber.go
│   │   └── ports/             # Port interfaces
│   │       ├── config.go
│   │       └── repository.go
│   └── service/               # Business logic services
│       ├── permissions/       # Permission deployment and checks
│       ├── subscribers/      # Subscriber management
│       └── tracer/           # Trace message handling
│           └── tracer_service.go
├── assets/
│   ├── embed_files.go         # Embedded asset management
│   ├── ins/
│   │   └── Omni_Initialize.ins  # Initialization script
│   └── sql/
│       ├── Omni_Tracer.sql    # Main tracer PL/SQL package
│       └── Permission_Checks.sql
├── scripts/
│   ├── setup_odpi.py          # ODPI-C library setup script
│   ├── delete_queues.sql     # Cleanup script
│   └── restart_ora_listner.sh
├── third_party/
│   └── odpi/                  # ODPI-C library (Oracle DB driver for C)
│       ├── include/
│       ├── src/
│       └── lib/
├── docs/                      # Architecture documentation
├── resources/                 # Application resources
├── Makefile                   # Build automation
├── go.mod                     # Go module definition
└── omniview.bolt              # Local database (created on first run)
```

## Prerequisites

Before building OmniView, ensure you have the following installed:

### Required Software

| Requirement | Version | Description |
|-------------|---------|-------------|
| **Go** | 1.24+ | Programming language |
| **Oracle Instant Client** | 23.7+ | Oracle database client libraries |
| **GCC/Clang** | Any recent version | C compiler for CGO |
| **make** | Any recent version | Build automation |

### Platform-Specific Requirements

#### macOS (Apple Silicon)
- Oracle Instant Client for ARM64
- Download from: [Oracle Instant Client macOS ARM64](https://www.oracle.com/database/technologies/instant-client/macos-arm64-downloads.html)

#### Windows
- Oracle Instant Client for x64
- Download from: [Oracle Instant Client Windows x64](https://www.oracle.com/database/technologies/instant-client/winx64-64-downloads.html)

#### Linux
- Oracle Instant Client for Linux x64

## Building from Source

### Quick Start (Recommended)

The easiest way to set up and build the project is using the provided setup script:

```bash
# Clone the repository
git clone https://github.com/BasuruK/OmniInspect.git
cd OmniInspect

# Run the setup script with automatic build
python scripts/setup_odpi.py --make

# Run the application
./omniview
```

### Manual Build Steps

If you prefer to build manually or need more control:

#### 1. Install Oracle Instant Client

Download and extract Oracle Instant Client to the appropriate directory:

| Platform | Path | Download URL |
|----------|------|--------------|
| macOS ARM64 | `/opt/oracle/instantclient_23_7` | [Link](https://www.oracle.com/database/technologies/instant-client/macos-arm64-downloads.html) |
| Windows x64 | `C:\oracle_inst\instantclient_23_7` | [Link](https://www.oracle.com/database/technologies/instant-client/winx64-64-downloads.html) |
| Linux x64 | `/opt/oracle/instantclient_23_7` | [Link](https://www.oracle.com/database/technologies/instant-client/linux-x64-downloads.html) |

#### 2. Set Up ODPI-C Library

The ODPI-C library provides Go bindings for Oracle Database. Run the setup script:

```bash
# Without building (just setup)
python scripts/setup_odpi.py

# Or with automatic build
python scripts/setup_odpi.py --make
```

This script will:
- Download ODPI-C v5.6.4 from GitHub
- Extract header files to `third_party/odpi/include/`
- Copy source files to `third_party/odpi/src/`
- Build the ODPI-C shared library

#### 3. Build the Application

Use the Makefile to build:

```bash
# Build the application
make build

# Or with a specific version
make build VERSION=v1.0.0

# Build and run
make run
```

#### 4. Run the Application

```bash
./omniview
```

On first run, the application will:
1. Prompt you to enter your Oracle database connection details (host, port, database name, username, password)
2. Create a local BoltDB database file (`omniview.bolt`) to store your settings
3. Deploy the OMNI_TRACER_API package to your Oracle schema
4. Initialize the tracer queue
5. Register a subscriber
6. Start listening for trace messages

#### Switching to a Different Database

If you need to connect to a different database:
1. Delete the `omniview.bolt` file
2. Re-run the application
3. Enter the new database connection details when prompted

*(Note: This process will be simplified in a future update.)*

## Makefile Targets

| Target | Description |
|--------|-------------|
| `make build` | Build the Go application |
| `make build VERSION=x.x.x` | Build with specific version |
| `make run` | Build and run the application |
| `make odpi` | Build only ODPI-C library |
| `make deps` | Check/build dependencies |
| `make clean` | Remove all build artifacts |
| `make test` | Run tests |
| `make install` | Install Go dependencies |
| `make release VERSION=x.x.x` | Build and package for distribution |
| `make help` | Show available targets |

## Architecture

OmniView uses a Hexagonal (Ports and Adapters) architecture:

```
┌─────────────────────────────────────────────────────────────────┐
│                      Application Bootstrap                      │
│                    (cmd/omniview/main.go)                       │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                        Services Layer                           │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐           │
│  │   Tracer     │  │  Permission  │  │  Subscriber  │           │
│  │   Service    │  │   Service    │  │   Service    │           │
│  └──────────────┘  └──────────────┘  └──────────────┘           │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Adapters Layer                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐           │
│  │   Oracle     │  │   BoltDB     │  │    Config    │           │
│  │   Adapter    │  │   Adapter    │  │   Loader     │           │
│  └──────────────┘  └──────────────┘  └──────────────┘           │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    External Systems                             │
│  ┌──────────────┐  ┌──────────────┐                             │
│  │    Oracle    │  │    Local     │                             │
│  │  Database    │  │    Storage   │                             │
│  └──────────────┘  └──────────────┘                             │
└─────────────────────────────────────────────────────────────────┘
```

## Message Flow

```
┌─────────────────┐     Trace_Message()      ┌─────────────────┐
│  Oracle PL/SQL  │ ────────────────────────▶│  OMNI_TRACER    │
│     Code        │                          │     QUEUE       │
└─────────────────┘                          └────────┬────────┘
                                                           │
                                                           │ Dequeue
                                                           ▼
┌─────────────────┐     Display            ┌─────────────────┐
│  OmniView TUI   │ ◀──────────────────────│   Blocking      │
│   (Console)     │                        │   Consumer      │
└─────────────────┘                        └─────────────────┘
```

## Contributing

Contributions are welcome. Please feel free to submit a Pull Request.

## License

Copyright (c) 2025 Basuru Balasuriya. All Rights Reserved.

This software is the exclusive property of Basuru Balasuriya ("the Author"). See the [LICENSE](LICENSE) file for full terms and conditions.

## Star History

<p align="center">
  <a href="https://star-history.com/#BasuruK/OmniInspect">
    <img src="https://api.star-history.com/svg?repos=BasuruK/OmniInspect&type=Date" alt="Star History">
  </a>
</p>

## Citations

If you use OmniView in your research or project, please cite it using the following format:

### BibTeX

```bibtex
@software{Balasiuriya2026OmniView,
  author  = {Basuru Balasuriya},
  title   = {OmniView: Oracle Database Message Passing TUI Application},
  year    = {2026},
  url     = {https://github.com/BasuruK/OmniInspect},
  license = {Proprietary}
}
```

### APA Style

Balasuriya, B. (2026). *OmniView: Oracle Database Message Passing TUI Application* (Version 1.0.0) [Software]. Retrieved from https://github.com/BasuruK/OmniInspect

### Chicago Style

Balasuriya, Basuru. 2026. *OmniView: Oracle Database Message Passing TUI Application*. Software. https://github.com/BasuruK/OmniInspect.

---

<p align="center">
  Built with 💖, by Basuru Balasuriya
</p>
