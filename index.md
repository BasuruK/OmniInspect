# OmniView Project Index

## Documentation

### Core Documentation

- **[README.md](./README.md)** - Product overview, prerequisites, build flow, and architecture summary
- **[AGENTS.md](./AGENTS.md)** - Contributor and agent implementation guidance
- **[DESIGN.md](./DESIGN.md)** - Target-state Bubble Tea and Lip Gloss UI design contract
- **[Makefile](./Makefile)** - Build automation with CGO and Oracle client configuration
- **[go.mod](./go.mod)** - Go module definition
- **[go.sum](./go.sum)** - Go dependencies checksum

### docs/

- **[docs/index.md](./docs/index.md)** - Documentation index
- **[docs/project-overview.md](./docs/project-overview.md)** - Executive summary and classification
- **[docs/architecture.md](./docs/architecture.md)** - Technical architecture and runtime flows
- **[docs/source-tree-analysis.md](./docs/source-tree-analysis.md)** - Annotated repository structure
- **[docs/component-inventory.md](./docs/component-inventory.md)** - TUI screens and reusable UI building blocks
- **[docs/development-guide.md](./docs/development-guide.md)** - Local setup, build, run, and test workflow
- **[docs/deployment-guide.md](./docs/deployment-guide.md)** - Build packaging and runtime deployment considerations
- **[docs/contribution-guide.md](./docs/contribution-guide.md)** - Coding boundaries and contributor expectations

### Design Notes (docs/)

- **[docs/ARCHITECTURE_AND_MULTI_SUBSCRIBER_PLAN.md](./docs/ARCHITECTURE_AND_MULTI_SUBSCRIBER_PLAN.md)** - Multi-subscriber evolution planning
- **[docs/BLOCKING_DEQUEUE_IMPLEMENTATION.md](./docs/BLOCKING_DEQUEUE_IMPLEMENTATION.md)** - Oracle blocking dequeue notes
- **[docs/SELF_UPDATER_IMPLEMENTATION.md](./docs/SELF_UPDATER_IMPLEMENTATION.md)** - Updater architecture notes
- **[docs/SUBSCRIBER_ISOLATION_SOLUTION.md](./docs/SUBSCRIBER_ISOLATION_SOLUTION.md)** - Subscriber isolation design

## Source Code

### cmd/

- **[cmd/omniview/main.go](./cmd/omniview/main.go)** - Application entry point and composition root

### internal/core/domain/

- **[internal/core/domain/errors.go](./internal/core/domain/errors.go)** - Sentinel errors
- **[internal/core/domain/subscriber.go](./internal/core/domain/subscriber.go)** - Subscriber entity
- **[internal/core/domain/queue_message.go](./internal/core/domain/queue_message.go)** - Queue message entity
- **[internal/core/domain/database_settings.go](./internal/core/domain/database_settings.go)** - Database settings value object
- **[internal/core/domain/permissions.go](./internal/core/domain/permissions.go)** - Permissions entity
- **[internal/core/domain/config.go](./internal/core/domain/config.go)** - Configuration value objects
- **[internal/core/domain/webhook.go](./internal/core/domain/webhook.go)** - Webhook configuration

### internal/core/ports/

- **[internal/core/ports/repository.go](./internal/core/ports/repository.go)** - Repository interfaces
- **[internal/core/ports/config.go](./internal/core/ports/config.go)** - Configuration interfaces

### internal/service/

- **[internal/service/tracer/tracer_service.go](./internal/service/tracer/tracer_service.go)** - Tracer business logic
- **[internal/service/updater/updater_service.go](./internal/service/updater/updater_service.go)** - Updater service
- **[internal/service/webhook/webhook_service.go](./internal/service/webhook/webhook_service.go)** - Webhook forwarding
- **[internal/service/permissions/permissions_service.go](./internal/service/permissions/permissions_service.go)** - Permission checking
- **[internal/service/subscribers/subscriber_service.go](./internal/service/subscribers/subscriber_service.go)** - Subscriber management

### internal/adapter/ui/

- **[internal/adapter/ui/ui.go](./internal/adapter/ui/ui.go)** - UI adapter entry point
- **[internal/adapter/ui/model.go](./internal/adapter/ui/model.go)** - Bubble Tea model
- **[internal/adapter/ui/messages.go](./internal/adapter/ui/messages.go)** - Message handlers
- **[internal/adapter/ui/main_screen.go](./internal/adapter/ui/main_screen.go)** - Main screen view
- **[internal/adapter/ui/welcome.go](./internal/adapter/ui/welcome.go)** - Welcome screen
- **[internal/adapter/ui/onboarding.go](./internal/adapter/ui/onboarding.go)** - Onboarding flow
- **[internal/adapter/ui/loading.go](./internal/adapter/ui/loading.go)** - Loading state
- **[internal/adapter/ui/chrome.go](./internal/adapter/ui/chrome.go)** - Application chrome
- **[internal/adapter/ui/commands.go](./internal/adapter/ui/commands.go)** - Command handling
- **[internal/adapter/ui/add_database_form.go](./internal/adapter/ui/add_database_form.go)** - Add database form
- **[internal/adapter/ui/database_list.go](./internal/adapter/ui/database_list.go)** - Database list view
- **[internal/adapter/ui/database_factory.go](./internal/adapter/ui/database_factory.go)** - Database factory
- **[internal/adapter/ui/database_settings.go](./internal/adapter/ui/database_settings.go)** - Database settings form
- **[internal/adapter/ui/webhook_settings.go](./internal/adapter/ui/webhook_settings.go)** - Webhook settings form
- **[internal/adapter/ui/styles/styles.go](./internal/adapter/ui/styles/styles.go)** - Lip Gloss styles
- **[internal/adapter/ui/animations/omniview_logo_anim.go](./internal/adapter/ui/animations/omniview_logo_anim.go)** - Logo animation

### internal/adapter/storage/oracle/

- **[internal/adapter/storage/oracle/oracle_adapter.go](./internal/adapter/storage/oracle/oracle_adapter.go)** - Oracle adapter
- **[internal/adapter/storage/oracle/subscriptions.go](./internal/adapter/storage/oracle/subscriptions.go)** - Subscription management
- **[internal/adapter/storage/oracle/queue.go](./internal/adapter/storage/oracle/queue.go)** - Queue operations
- **[internal/adapter/storage/oracle/sql_parse.go](./internal/adapter/storage/oracle/sql_parse.go)** - SQL parsing
- **[internal/adapter/storage/oracle/dequeue_ops.c](./internal/adapter/storage/oracle/dequeue_ops.c)** - C dequeue operations
- **[internal/adapter/storage/oracle/dequeue_ops.h](./internal/adapter/storage/oracle/dequeue_ops.h)** - C dequeue header

### internal/adapter/storage/boltdb/

- **[internal/adapter/storage/boltdb/bolt_adapter.go](./internal/adapter/storage/boltdb/bolt_adapter.go)** - BoltDB adapter
- **[internal/adapter/storage/boltdb/database_settings_repository.go](./internal/adapter/storage/boltdb/database_settings_repository.go)** - Database settings persistence
- **[internal/adapter/storage/boltdb/subscriber_repository.go](./internal/adapter/storage/boltdb/subscriber_repository.go)** - Subscriber persistence
- **[internal/adapter/storage/boltdb/permissions_repository.go](./internal/adapter/storage/boltdb/permissions_repository.go)** - Permissions persistence

### internal/adapter/config/

- **[internal/adapter/config/settings_loader.go](./internal/adapter/config/settings_loader.go)** - Settings loader

### internal/app/

- **[internal/app/app.go](./internal/app/app.go)** - Application object

### internal/updater/

- **[internal/updater/updater.go](./internal/updater/updater.go)** - Self-update logic

## Scripts

- **[scripts/debug.sqlnb](./scripts/debug.sqlnb)** - Debug notebook
- **[scripts/delete_queues.sql](./scripts/delete_queues.sql)** - Queue deletion script
- **[scripts/restart_ora_listener.sh](./scripts/restart_ora_listener.sh)** - Oracle listener restart
- **[scripts/setup_odpi.py](./scripts/setup_odpi.py)** - ODPI setup

## Assets

### assets/sql/

- **[assets/sql/Omni_Tracer.sql](./assets/sql/Omni_Tracer.sql)** - Trace PL/SQL
- **[assets/sql/Permission_Checks.sql](./assets/sql/Permission_Checks.sql)** - Permission checks

### assets/ins/

- **[assets/ins/Omni_Initialize.ins](./assets/ins/Omni_Initialize.ins)** - Initialization script

### assets/

- **[assets/embed_files.go](./assets/embed_files.go)** - Embedded file assets

## Resources

- **[resources/icon.png](./resources/icon.png)** - Icon image
- **[resources/icon.ico](./resources/icon.ico)** - Windows icon
- **[resources/omniview.png](./resources/omniview.png)** - OmniView logo

## Configuration

- **[settings.json](./settings.json)** - VS Code settings
- **[skills-lock.json](./skills-lock.json)** - Skills lock file
- ** [.gitignore](./.gitignore)** - Git ignore patterns
