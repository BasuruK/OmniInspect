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
    MKDIR = if not exist $(subst /,\,$1) mkdir $(subst /,\,$1)
    RM = rmdir /s /q
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
    MKDIR = mkdir -p $1
    RM = rm -rf
endif

all: $(TARGET)

$(ODPI_BASE)/build/%.o: $(ODPI_BASE)/src/%.c | $(ODPI_BASE)/build
	$(CC) $(CFLAGS) -c $< -o $@

$(ODPI_BASE)/build:
ifeq ($(DETECTED_OS),Windows)
	@if not exist "$(subst /,\,$(ODPI_BASE)\build)" mkdir "$(subst /,\,$(ODPI_BASE)\build)"
else
	@mkdir -p $(ODPI_BASE)/build
endif

$(ODPI_BASE)/lib:
ifeq ($(DETECTED_OS),Windows)
	@if not exist "$(subst /,\,$(ODPI_BASE)\lib)" mkdir "$(subst /,\,$(ODPI_BASE)\lib)"
else
	@mkdir -p $(ODPI_BASE)/lib
endif

$(TARGET): $(ODPI_OBJ) | $(ODPI_BASE)/lib
	$(CC) $(LDFLAGS) -o $@ $^ $(LIBS)
ifeq ($(DETECTED_OS),Windows)
	@copy /Y "$(subst /,\,$(TARGET))" . >nul && echo Copied odpi.dll to workspace root
endif

clean:
ifeq ($(DETECTED_OS),Windows)
	@if exist "$(subst /,\,$(ODPI_BASE)\build)" rmdir /s /q "$(subst /,\,$(ODPI_BASE)\build)"
	@if exist "$(subst /,\,$(ODPI_BASE)\src)" del /f /q "$(subst /,\,$(ODPI_BASE)\src\*.c)" "$(subst /,\,$(ODPI_BASE)\src\*.h)"
	@if exist odpi.dll del /f /q odpi.dll
else
	@rm -rf $(ODPI_BASE)/build
	@rm -rf $(ODPI_BASE)/src
	@rm -f odpi.dll
endif

# Phony targets
.PHONY: all clean