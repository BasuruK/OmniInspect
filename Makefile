# Detect OS - use OS environment variable on Windows
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

# Updated paths for DDD architecture
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

all: $(TARGET)

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

clean:
ifeq ($(DETECTED_OS),Windows)
    ifeq ($(USE_BASH),1)
	@$(RM_CMD) $(ODPI_BASE)/build
	@$(RM_CMD) $(ODPI_BASE)/src
	@rm -f odpi.dll
    else
	@if exist $(subst /,\,$(ODPI_BASE)\build) $(RM_CMD) $(subst /,\,$(ODPI_BASE)\build)
	@if exist $(subst /,\,$(ODPI_BASE)\src) $(RM_CMD) $(subst /,\,$(ODPI_BASE)\src)
    endif
else
	@$(RM_CMD) $(ODPI_BASE)/build
	@$(RM_CMD) $(ODPI_BASE)/src
endif

# Phony targets
.PHONY: all clean