# ============================================================================
# OS Detection
# ============================================================================
ifeq ($(OS),Windows_NT)
    DETECTED_OS := Windows
else
    UNAME_S := $(shell uname -s)
    ifeq ($(UNAME_S),Darwin)
        DETECTED_OS := MacOS
    else
        DETECTED_OS := $(UNAME_S)
    endif
endif

# Detect shell on Windows
ifeq ($(DETECTED_OS),Windows)
    # Check if we're running in bash (Git Bash, MSYS2, WSL, etc.)
    SHELL_TYPE := $(shell echo $$0)
    ifneq (,$(findstring bash,$(SHELL_TYPE)))
        USE_BASH := 1
    else ifneq (,$(findstring sh,$(SHELL_TYPE)))
        USE_BASH := 1
    else
        USE_BASH := 0
    endif
endif

# ============================================================================
# ODPI-C Library Build Configuration
# ============================================================================
ODPI_BASE = third_party/odpi
ODPI_SRC = $(wildcard $(ODPI_BASE)/src/*.c)
ODPI_OBJ = $(patsubst $(ODPI_BASE)/src/%.c,$(ODPI_BASE)/build/%.o,$(ODPI_SRC))

INCLUDE = -I$(ODPI_BASE)/include

ifeq ($(DETECTED_OS),Windows)
    # Windows settings
    INSTANT_CLIENT_DIR ?= C:/oracle_inst/instantclient_23_7
    INSTANT_CLIENT_SDK = $(INSTANT_CLIENT_DIR)/sdk
    LIBS = -L$(INSTANT_CLIENT_DIR) -loci
    CFLAGS = -O2 -Wall $(INCLUDE) -I$(INSTANT_CLIENT_SDK)/include
    LDFLAGS = -shared
    TARGET = $(ODPI_BASE)/lib/odpi.dll
    
    # Set commands based on shell type
    ifeq ($(USE_BASH),1)
        # Bash-compatible commands (Git Bash, MSYS2, etc.)
        MKDIR_CMD = mkdir -p
        COPY_CMD = cp -f
        RM_CMD = rm -rf
    else
        # Native Windows commands (cmd.exe, PowerShell)
        MKDIR_CMD = mkdir
        COPY_CMD = copy /Y
        RM_CMD = rmdir /s /q
        DEL_CMD = del /f /q
    endif
else ifeq ($(DETECTED_OS),MacOS)
    # MacOS settings
    INSTANT_CLIENT_DIR ?= /opt/oracle/instantclient_23_7
    INSTANT_CLIENT_INCLUDE = $(INSTANT_CLIENT_DIR)/sdk/include
    LDFLAGS = -dynamiclib -arch arm64 \
        -install_name @rpath/libodpi.dylib \
        -Wl,-rpath,$(INSTANT_CLIENT_DIR)
    CFLAGS = -O2 -Wall $(INCLUDE) -I$(INSTANT_CLIENT_INCLUDE) -arch arm64
    TARGET = $(ODPI_BASE)/lib/libodpi.dylib
    LIBS = -L$(INSTANT_CLIENT_DIR) -lclntsh
    MKDIR_CMD = mkdir -p
    RM_CMD = rm -rf
endif

# ============================================================================
# Go Application Build Configuration
# ============================================================================
BINARY_NAME = omniview
BUILD_DIR = .
MAIN_PATH = cmd/omniview
VERSION ?= dev
GO_LDFLAGS = -ldflags "-X OmniView/internal/app.Version=$(VERSION)"

# Export CGO flags for Go build (picked up automatically by go build)
ifeq ($(DETECTED_OS),MacOS)
    export CGO_CFLAGS = -I$(PWD)/$(ODPI_BASE)/include
    export CGO_LDFLAGS = -L$(PWD)/$(ODPI_BASE)/lib -lodpi -Wl,-rpath,$(PWD)/$(ODPI_BASE)/lib -Wl,-rpath,$(INSTANT_CLIENT_DIR)
else ifeq ($(DETECTED_OS),Windows)
    export CGO_CFLAGS = -I$(PWD)/$(ODPI_BASE)/include
    export CGO_LDFLAGS = -L$(PWD)/$(ODPI_BASE)/lib -lodpi -L$(INSTANT_CLIENT_DIR) -loci
endif

# ============================================================================
# Build Targets
# ============================================================================

# Default target: Build the Go application
.DEFAULT_GOAL := build

# Build ODPI-C library only
.PHONY: odpi
odpi: $(TARGET)

$(ODPI_BASE)/build/%.o: $(ODPI_BASE)/src/%.c | $(ODPI_BASE)/build
	$(CC) $(CFLAGS) -c $< -o $@

$(ODPI_BASE)/build:
ifeq ($(DETECTED_OS),Windows)
    ifeq ($(USE_BASH),1)
	@$(MKDIR_CMD) $(ODPI_BASE)/build
    else
	@if not exist $(subst /,\,$(ODPI_BASE)\build) $(MKDIR_CMD) $(subst /,\,$(ODPI_BASE)\build)
    endif
else
	@$(MKDIR_CMD) $(ODPI_BASE)/build
endif

$(ODPI_BASE)/lib:
ifeq ($(DETECTED_OS),Windows)
    ifeq ($(USE_BASH),1)
	@$(MKDIR_CMD) $(ODPI_BASE)/lib
    else
	@if not exist $(subst /,\,$(ODPI_BASE)\lib) $(MKDIR_CMD) $(subst /,\,$(ODPI_BASE)\lib)
    endif
else
	@$(MKDIR_CMD) $(ODPI_BASE)/lib
endif

$(TARGET): $(ODPI_OBJ) | $(ODPI_BASE)/lib
	$(CC) $(LDFLAGS) -o $@ $^ $(LIBS)
ifeq ($(DETECTED_OS),Windows)
    ifeq ($(USE_BASH),1)
	@$(COPY_CMD) $(TARGET) . && echo "Copied odpi.dll to workspace root"
    else
	@$(COPY_CMD) $(subst /,\,$(TARGET)) . >nul && echo Copied odpi.dll to workspace root
    endif
endif

# Check dependencies before building Go app
.PHONY: deps
deps: $(TARGET)
	@echo "[INFO] Checking dependencies..."
	@echo "[OK] All dependencies ready"

# Build Go application (CGO will compile subscription/*.c automatically)
.PHONY: build
build: deps
	@echo "[BUILD] Building $(BINARY_NAME) (version: $(VERSION))..."
	@echo "   CGO_CFLAGS=$(CGO_CFLAGS)"
	@echo "   CGO_LDFLAGS=$(CGO_LDFLAGS)"
	go build -v $(GO_LDFLAGS) -o $(BINARY_NAME) ./$(MAIN_PATH)
	@echo "[OK] Build complete: $(BINARY_NAME) ($(VERSION))"

# Run the application
.PHONY: run
run: build
	@echo "[RUN] Running $(BINARY_NAME)..."
	ifeq ($(DETECTED_OS),Windows)
		./$(BINARY_NAME).exe
	else
		./$(BINARY_NAME)
	endif

# Run tests
.PHONY: test
test:
	@echo "[TEST] Running tests..."
	go test -v ./...

# Debug CGO compilation
.PHONY: check-cgo
check-cgo:
	@echo "[DEBUG] Checking CGO compilation..."
	@echo "CGO_CFLAGS=$(CGO_CFLAGS)"
	@echo "CGO_LDFLAGS=$(CGO_LDFLAGS)"
	@echo ""
	@echo "Building subscription package with debug output:"
	@go build -x ./internal/adapter/subscription 2>&1 | grep -E "gcc|clang|queue_callback" || true
	@echo "Building oracle storage package with debug output:"
	@go build -x ./internal/adapter/storage/oracle 2>&1 | grep -E "gcc|clang|dequeue_ops" || true

# Install Go dependencies
.PHONY: install
install:
	@echo "[INSTALL] Installing Go dependencies..."
	go mod download
	go mod tidy
	@echo "[OK] Dependencies installed"

# Format code
.PHONY: fmt
fmt:
	@echo "[FMT] Formatting code..."
	go fmt ./...
	@echo "[OK] Format complete"

# Lint code
.PHONY: lint
lint:
	@echo "[LINT] Linting code..."
	go vet ./...
	@echo "[OK] Lint complete"

# ============================================================================
# Release Packaging
# ============================================================================
.PHONY: release
release: build
	@echo "[RELEASE] Packaging release $(VERSION)..."
ifeq ($(DETECTED_OS),Windows)
	@echo "[RELEASE] Creating Windows amd64 archive..."
    ifeq ($(USE_BASH),1)
	@mkdir -p release
	@cp $(BINARY_NAME).exe release/ 2>/dev/null || cp $(BINARY_NAME) release/
	@cp odpi.dll release/ 2>/dev/null || true
	@cd release && zip -r ../omniview-windows-amd64-$(VERSION).zip . && cd ..
	@rm -rf release
    else
	@if not exist release $(MKDIR_CMD) release
	@$(COPY_CMD) $(BINARY_NAME).exe release\ >nul 2>&1 || $(COPY_CMD) $(BINARY_NAME) release\ >nul
	@$(COPY_CMD) odpi.dll release\ >nul 2>&1
	@powershell -Command "Compress-Archive -Path 'release\*' -DestinationPath 'omniview-windows-amd64-$(VERSION).zip' -Force"
	@$(RM_CMD) release
    endif
	@echo "[OK] Created omniview-windows-amd64-$(VERSION).zip"
else ifeq ($(DETECTED_OS),MacOS)
	@echo "[RELEASE] Creating macOS arm64 archive..."
	@mkdir -p release
	@cp $(BINARY_NAME) release/
	@cp $(ODPI_BASE)/lib/libodpi.dylib release/ 2>/dev/null || true
	@tar -czf omniview-darwin-arm64-$(VERSION).tar.gz -C release .
	@rm -rf release
	@echo "[OK] Created omniview-darwin-arm64-$(VERSION).tar.gz"
endif

# Clean all build artifacts
.PHONY: clean
clean:
	@echo "[CLEAN] Cleaning build artifacts..."
ifeq ($(DETECTED_OS),Windows)
    ifeq ($(USE_BASH),1)
	@$(RM_CMD) $(ODPI_BASE)/build
	@$(RM_CMD) $(ODPI_BASE)/src
	@rm -f $(BINARY_NAME) $(BINARY_NAME).exe odpi.dll *.db
    else
	@if exist $(subst /,\,$(ODPI_BASE)\build) $(RM_CMD) $(subst /,\,$(ODPI_BASE)\build)
	@if exist $(subst /,\,$(ODPI_BASE)\src) $(RM_CMD) $(subst /,\,$(ODPI_BASE)\src)
	@if exist $(BINARY_NAME).exe $(DEL_CMD) $(BINARY_NAME).exe
	@if exist *.db $(DEL_CMD) *.db
    endif
else
	@$(RM_CMD) $(ODPI_BASE)/build
	@$(RM_CMD) $(ODPI_BASE)/src
	@rm -f $(BINARY_NAME) *.db
endif
	@go clean -cache
	@echo "[OK] Clean complete"

# Help
.PHONY: help
help:
	@echo "OmniInspect Makefile - Available targets:"
	@echo ""
	@echo "  make build                  - Build the Go application (default)"
	@echo "  make build VERSION=v1.0.0   - Build with specific version"
	@echo "  make run                    - Build and run the application"
	@echo "  make release VERSION=v1.0.0 - Build and package for distribution"
	@echo "  make odpi                   - Build only ODPI-C library"
	@echo "  make deps                   - Check/build dependencies"
	@echo "  make clean                  - Remove all build artifacts"
	@echo "  make test                   - Run tests"
	@echo "  make check-cgo              - Debug CGO compilation"
	@echo "  make install                - Install Go dependencies"
	@echo "  make fmt                    - Format Go code"
	@echo "  make lint                   - Lint Go code"
	@echo "  make help                   - Show this help message"

# Phony targets
.PHONY: all clean odpi deps build run test check-cgo install fmt lint release help