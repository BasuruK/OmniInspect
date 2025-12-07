UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)

ODPI_SRC = $(wildcard src/*.c)
ODPI_OBJ = $(patsubst src/%.c,build/%.o,$(ODPI_SRC))
INCLUDE = -Iinclude

ifeq ($(UNAME_S),Windows_NT)
    # Windows settings
    INSTANT_CLIENT_DIR = C:/oracle_inst/instantclient_23_7/sdk/lib/msvc
    LIBS = -L$(INSTANT_CLIENT_DIR) -loci
    CFLAGS = -O2 -Wall $(INCLUDE)
    LDFLAGS = -shared
    TARGET = lib/odpi.dll
    COPY_TARGET = ../../../odpi.dll
else
    # MacOS settings
    INSTANT_CLIENT_DIR = /opt/oracle/instantclient_23_7/sdk/lib
    LDFLAGS = -dynamiclib -arch arm64
    TARGET = lib/libodpi.dylib
    LIBS = -L$(INSTANT_CLIENT_DIR) -lclntsh
    TARGET = lib/libodpi.dylib
    COPY_TARGET = ../../../libodpi.dylib
endif

all: $(TARGET)

build/%.o: src/%.c | build
	$(CC) $(CFLAGS) -c $< -o $@

build:
	mkdir -p build

lib:
	mkdir -p lib

$(TARGET): $(ODPI_OBJ) | lib
	$(CC) $(LDFLAGS) -o $@ $^ $(LIBS)
	cp $(TARGET) $(COPY_TARGET)

clean:
	rm -rf build
	rm -rf src

# Phony targets
.PHONY: all clean