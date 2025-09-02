ODPI_SRC = $(wildcard src/*.c)
ODPI_OBJ = $(patsubst src/%.c,build/%.o,$(ODPI_SRC))
INSTANT_CLIENT_DIR = C:/oracle_inst/instantclient_23_7/sdk/lib/msvc
INCLUDE = -Iinclude
LIBS = -L$(INSTANT_CLIENT_DIR) -loci
CFLAGS = -O2 -Wall $(INCLUDE)
LDFLAGS = -shared

all: lib/odpi.dll

build/%.o: src/%.c | build
	$(CC) $(CFLAGS) -c $< -o $@

build:
	mkdir -p build

lib:
	mkdir -p lib

lib/odpi.dll: $(ODPI_OBJ) | lib
	$(CC) $(LDFLAGS) -o $@ $^ $(LIBS)
	copy lib\odpi.dll ..\..\..\odpi.dll

clean:
	if exist build rmdir /s /q build
	if exist src rmdir /s /q src