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
	@if [ ! -f "$(TARGET)" ]; then \
		echo "[WARN] ODPI-C library not found. Building..."; \
		$(MAKE) odpi; \
	fi
	@echo "[OK] All dependencies ready"

# Build Go application (CGO will compile subscription/*.c automatically)
.PHONY: build
build: deps
	@echo "[BUILD] Building $(BINARY_NAME)..."
	@echo "   CGO_CFLAGS=$(CGO_CFLAGS)"
	@echo "   CGO_LDFLAGS=$(CGO_LDFLAGS)"
	go build -v -o $(BINARY_NAME) ./$(MAIN_PATH)
	@echo "[OK] Build complete: $(BINARY_NAME)"

# Run the application
.PHONY: run
run: build
	@echo "[RUN] Running $(BINARY_NAME)..."
	./$(BINARY_NAME)

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
	@if exist odpi.dll $(DEL_CMD) odpi.dll
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
	@echo "  make build       - Build the Go application (default)"
	@echo "  make run         - Build and run the application"
	@echo "  make odpi        - Build only ODPI-C library"
	@echo "  make deps        - Check/build dependencies"
	@echo "  make clean       - Remove all build artifacts"
	@echo "  make test        - Run tests"
	@echo "  make check-cgo   - Debug CGO compilation"
	@echo "  make install     - Install Go dependencies"
	@echo "  make fmt         - Format Go code"
	@echo "  make lint        - Lint Go code"
	@echo "  make help        - Show this help message"

# Phony targets
.PHONY: all clean odpi deps build run test check-cgo install fmt lint help