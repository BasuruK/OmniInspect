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

ODPI_SRC = $(wildcard src/*.c)
ODPI_OBJ = $(patsubst src/%.c,build/%.o,$(ODPI_SRC))
INCLUDE = -Iinclude

ifeq ($(DETECTED_OS),Windows)
    # Windows settings
    INSTANT_CLIENT_DIR = C:/oracle_inst/instantclient_23_7/sdk/lib/msvc
    LIBS = -L$(INSTANT_CLIENT_DIR) -loci
    CFLAGS = -O2 -Wall $(INCLUDE)
    LDFLAGS = -shared
    TARGET = lib/odpi.dll
    COPY_TARGET = ../../../odpi.dll
    MKDIR = if not exist $(subst /,\,$1) mkdir $(subst /,\,$1)
    RM = rmdir /s /q
else
    # MacOS settings
    INSTANT_CLIENT_DIR = /opt/oracle/instantclient_23_7/sdk/lib
    LDFLAGS = -dynamiclib -arch arm64
    CFLAGS = -O2 -Wall $(INCLUDE)
    TARGET = lib/libodpi.dylib
    LIBS = -L$(INSTANT_CLIENT_DIR) -lclntsh
    COPY_TARGET = ../../../libodpi.dylib
    MKDIR = mkdir -p $1
    RM = rm -rf
endif

all: $(TARGET)
	$(MAKE) clean

build/%.o: src/%.c | build
	$(CC) $(CFLAGS) -c $< -o $@

build:
ifeq ($(DETECTED_OS),Windows)
	@if not exist build mkdir build
else
	mkdir -p build
endif

lib:
ifeq ($(DETECTED_OS),Windows)
	@if not exist lib mkdir lib
else
	mkdir -p lib
endif

$(TARGET): $(ODPI_OBJ) | lib
	$(CC) $(LDFLAGS) -o $@ $^ $(LIBS)
ifeq ($(DETECTED_OS),Windows)
	copy /Y $(subst /,\,$(TARGET)) $(subst /,\,$(COPY_TARGET))
else
	cp $(TARGET) $(COPY_TARGET)
endif

clean:
ifeq ($(DETECTED_OS),Windows)
	@if exist build (rmdir /s /q build)
	@if exist lib (rmdir /s /q lib)
    @if exist src (rmdir /s /q src\*.c)
else
	rm -rf build
	rm -rf lib
    rm -rf src/*.c
endif

# Phony targets
.PHONY: all clean