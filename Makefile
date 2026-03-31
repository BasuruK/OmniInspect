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
        -install_name @executable_path/third_party/odpi/lib/libodpi.dylib \
        -Wl,-rpath,@executable_path/instantclient_23_7
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
# Note: The actual library linking is done in the Go files via #cgo directives
# For local development, use rpath to system Oracle Instant Client
# For release builds (RELEASE=1), use @executable_path for distribution
ifeq ($(DETECTED_OS),MacOS)
    export CGO_CFLAGS = -I$(PWD)/$(ODPI_BASE)/include
    ifeq ($(RELEASE),1)
        # Release build: use @executable_path for distribution
        export CGO_LDFLAGS = -Wl,-rpath,@executable_path -Wl,-rpath,@executable_path/third_party/odpi/lib -Wl,-rpath,@executable_path/instantclient_23_7
    else
        # Local dev build: use system Oracle Instant Client path
        export CGO_LDFLAGS = -Wl,-rpath,@executable_path -Wl,-rpath,@executable_path/third_party/odpi/lib -Wl,-rpath,$(INSTANT_CLIENT_DIR)
    endif
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
build: deps icon
	@echo "[BUILD] Building $(BINARY_NAME) (version: $(VERSION))..."
	@echo "   CGO_CFLAGS=$(CGO_CFLAGS)"
	@echo "   CGO_LDFLAGS=$(CGO_LDFLAGS)"
ifeq ($(DETECTED_OS),Windows)
	go build -v $(GO_LDFLAGS) -o $(BINARY_NAME).exe ./$(MAIN_PATH)
else
	go build -v $(GO_LDFLAGS) -o $(BINARY_NAME) ./$(MAIN_PATH)
endif
	@echo "[OK] Build complete: $(BINARY_NAME) ($(VERSION))"

# Generate icon .syso file for Windows executable
# Place your icon.ico file in the resources folder
.PHONY: icon
icon:
ifeq ($(DETECTED_OS),Windows)
	@echo "[ICON] Checking for icon resources..."
    ifeq ($(USE_BASH),1)
	@if test -e resources/icon.ico; then \
		echo "[ICON] Generating .syso file from resources/icon.ico..."; \
		go run github.com/akavel/rsrc@v0.10.2 -ico resources/icon.ico -o cmd/omniview/omniview.syso; \
		echo "[ICON] Icon embedded successfully"; \
	else \
		echo "[ICON] resources/icon.ico not found, skipping icon embedding"; \
	fi
    else
	@if exist resources\icon.ico ( \
		echo "[ICON] Generating .syso file from resources\icon.ico..." && \
		go run github.com/akavel/rsrc@v0.10.2 -ico resources\icon.ico -o cmd\omniview\omniview.syso && \
		echo "[ICON] Icon embedded successfully" \
	) else ( \
		echo "[ICON] resources\icon.ico not found, skipping icon embedding" \
	)
    endif
else
	@echo "[ICON] Icon embedding is Windows-only for now"
endif

# Clean icon artifacts
.PHONY: clean-icon
clean-icon:
ifeq ($(DETECTED_OS),Windows)
    ifeq ($(USE_BASH),1)
	@rm -f cmd/omniview/omniview.syso
	@echo "[OK] Icon artifacts cleaned"
    else
	@if exist cmd\omniview\omniview.syso del /f /q cmd\omniview\omniview.syso
	@echo "[OK] Icon artifacts cleaned"
    endif
else
	@echo "[INFO] No icon artifacts to clean on $(DETECTED_OS)"
endif

# Run the application
.PHONY: run
run: build
	@echo "[RUN] Running $(BINARY_NAME)..."
ifneq ($(DETECTED_OS),Windows)
	./$(BINARY_NAME)
else
	./$(BINARY_NAME).exe
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
# For release/publish, VERSION must be explicitly provided
# Strip 'v' prefix if present (for internal use in zip filenames)
# Use Make's built-in subst function (works cross-platform, unlike sed)
RELEASE_NUM := $(subst v,,$(VERSION))
# Add 'v' prefix for GitHub tag (used in publish)
RELEASE_TAG := v$(RELEASE_NUM)

.PHONY: release
release: build
	@echo "[RELEASE] Packaging release $(VERSION)..."
ifeq ($(DETECTED_OS),Windows)
	@echo "[RELEASE] Creating Windows amd64 archive..."
    ifeq ($(USE_BASH),1)
	@mkdir -p release
	@cp $(BINARY_NAME).exe release/ 2>/dev/null || cp $(BINARY_NAME) release/
	@cp odpi.dll release/ 2>/dev/null || true
	@cd release && zip -r ../omniview-windows-amd64-$(RELEASE_TAG).zip . && cd ..
	@sha256sum omniview-windows-amd64-$(RELEASE_TAG).zip | awk '{print $$1 "  " $$2}' > omniview-windows-amd64-$(RELEASE_TAG).zip.sha256
	@rm -rf release
    else
	@if not exist release $(MKDIR_CMD) release
	@$(COPY_CMD) $(BINARY_NAME).exe release\ >nul 2>&1 || $(COPY_CMD) $(BINARY_NAME) release\ >nul
	@$(COPY_CMD) odpi.dll release\ >nul 2>&1
	@powershell -Command "Compress-Archive -Path 'release\*' -DestinationPath 'omniview-windows-amd64-$(RELEASE_TAG).zip' -Force"
	@powershell -Command "$$h = (Get-FileHash -Algorithm SHA256 'omniview-windows-amd64-$(RELEASE_TAG).zip').Hash.ToLower(); \"$$h  omniview-windows-amd64-$(RELEASE_TAG).zip\" | Out-File -Encoding ascii -NoNewline 'omniview-windows-amd64-$(RELEASE_TAG).zip.sha256'"
	@$(RM_CMD) release
    endif
	@echo "[OK] Created omniview-windows-amd64-$(RELEASE_TAG).zip"
	@echo "[OK] Created omniview-windows-amd64-$(RELEASE_TAG).zip.sha256"
else ifeq ($(DETECTED_OS),MacOS)
	@echo "[RELEASE] Creating macOS arm64 archive..."
	@mkdir -p release
	@cp $(BINARY_NAME) release/
	@cp $(ODPI_BASE)/lib/libodpi.dylib release/ 2>/dev/null || true
	@tar -czf omniview-darwin-arm64-$(RELEASE_TAG).tar.gz -C release .
	@shasum -a 256 omniview-darwin-arm64-$(RELEASE_TAG).tar.gz | awk '{print $$1 "  " $$2}' > omniview-darwin-arm64-$(RELEASE_TAG).tar.gz.sha256
	@rm -rf release
	@echo "[OK] Created omniview-darwin-arm64-$(RELEASE_TAG).tar.gz"
	@echo "[OK] Created omniview-darwin-arm64-$(RELEASE_TAG).tar.gz.sha256"
endif

# Publish: Build binary, create git tag and push to remote
# Release artifacts are created by GitHub Actions (release.yml)
.PHONY: publish
publish:
ifneq ($(findstring dev,$(VERSION)),)
	@echo "[ERROR] VERSION must be provided for publish (e.g., make publish VERSION=1.0.0)"
	@exit 1
endif
	@echo "[PUBLISH] Building binary..."
	@$(MAKE) build VERSION=$(VERSION)
	@echo "[PUBLISH] Creating annotated tag $(RELEASE_TAG)..."
	@git tag -a $(RELEASE_TAG) -m "Release version $(RELEASE_NUM)"
	@echo "[PUBLISH] Pushing tag $(RELEASE_TAG) to origin..."
	@git push origin $(RELEASE_TAG)
	@echo "[OK] Published $(RELEASE_TAG) to remote"
	@echo "[INFO] GitHub workflow will now build and upload release artifacts"

# Clean all build artifacts
.PHONY: clean
clean:
	@echo "[CLEAN] Cleaning build artifacts..."
ifeq ($(DETECTED_OS),Windows)
    ifeq ($(USE_BASH),1)
	@$(RM_CMD) $(ODPI_BASE)/build
	@$(RM_CMD) $(ODPI_BASE)/src
	@rm -f $(BINARY_NAME) $(BINARY_NAME).exe odpi.dll *.db cmd/omniview/omniview.syso
    else
	@if exist $(subst /,\,$(ODPI_BASE)\build) $(RM_CMD) $(subst /,\,$(ODPI_BASE)\build)
	@if exist $(subst /,\,$(ODPI_BASE)\src) $(RM_CMD) $(subst /,\,$(ODPI_BASE)\src)
	@if exist $(BINARY_NAME).exe $(DEL_CMD) $(BINARY_NAME).exe
	@if exist cmd\omniview\omniview.syso $(DEL_CMD) cmd\omniview\omniview.syso
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
	@echo "  make build                        - Build the Go application (default)"
	@echo "  make build VERSION=v1.0.0         - Build with specific version"
	@echo "  make run                          - Build and run the application"
	@echo "  make release                      - Build and package for distribution"
	@echo "  make release VERSION=v1.0.0       - Package with specific version"
	@echo "  make publish VERSION=1.0.0        - Build, tag and push to remote (GitHub Actions handles packaging)"
	@echo "  make odpi                         - Build only ODPI-C library"
	@echo "  make deps                         - Check/build dependencies"
	@echo "  make clean                        - Remove all build artifacts"
	@echo "  make test                         - Run tests"
	@echo "  make check-cgo                    - Debug CGO compilation"
	@echo "  make install                      - Install Go dependencies"
	@echo "  make fmt                          - Format Go code"
	@echo "  make lint                         - Lint Go code"
	@echo "  make help                         - Show this help message"

# Phony targets
.PHONY: all clean odpi deps build run run-only test check-cgo install fmt lint release publish help