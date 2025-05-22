# OmniView

A Go-based application for Background Tracing activities.

## Getting Started

This section guides you through setting up your environment and installing OmniView.

### Prerequisites

Before you begin, ensure you have the following software installed and configured:

1.  **Go:**
    *   Version: 1.24 or higher (as specified in `go.mod`).
    *   Setup: A standard Go development environment. Ensure `GOROOT` and `GOPATH` are set, and Go's `bin` directory is in your system `PATH`.

2.  **C Compiler (for ODPI-C):**
    *   **Windows:** MinGW-w64 (e.g., via MSYS2 or by downloading from the MinGW-w64 site). Ensure `gcc.exe` and `make.exe` (or `mingw32-make.exe`) are in your system `PATH`.
    *   **Linux/macOS:** GCC and Make are typically pre-installed or available through package managers (e.g., `build-essential` on Debian/Ubuntu, Xcode Command Line Tools on macOS).

3.  **Oracle Instant Client:**
    *   This project requires Oracle Instant Client to connect to Oracle databases via ODPI-C.
    *   **Version:** The `internal/lib/odpi/Makefile` is configured for Instant Client version 23.7 (e.g., `instantclient_23_7`). You can download this or a compatible newer version.
    *   **Download:** Visit the official [Oracle Instant Client Downloads page](https://www.oracle.com/database/technologies/instant-client/downloads.html).
    *   **Required Packages (ZIP files):**
        *   **Basic Package** or **Basic Light Package**: Contains the core client libraries.
        *   **SDK Package**: Contains header files and libraries needed for compiling applications like ODPI-C.
    *   **Installation Steps:**
        1.  Create a directory for your Instant Client installation (e.g., `C:\oracle_inst\instantclient_23_7` on Windows or `/opt/oracle/instantclient_23_7` on Linux/macOS).
        2.  Unzip both the Basic (or Basic Light) and SDK packages into this directory. After extraction, you should have subdirectories like `sdk/include` and `sdk/lib/msvc` (on Windows) or `sdk/lib` (on Linux/macOS) within your Instant Client directory.
        3.  **Environment Variable:** Add the main Instant Client directory (e.g., `C:\oracle_inst\instantclient_23_7`) to your system's `PATH` environment variable. This allows the system to find the Oracle client DLLs/shared libraries at runtime.
            *   On Linux/macOS, you might also need to configure `LD_LIBRARY_PATH` or use `ldconfig` if the libraries are not in a standard location.

### Installation

Follow these steps to install and run OmniView:

1.  **Navigate to the Project Directory:**
    Assuming you have the project files already, navigate to the root directory of the project. For example:
    ```bash
    cd path/to/OmniView
    ```

2.  **Configure and Build `odpi.dll` (ODPI-C Shared Library):**
    *   OmniView uses ODPI-C, a C library, for Oracle Database connectivity. This requires compiling a shared library (`odpi.dll` on Windows, `odpi.so` on Linux, `odpi.dylib` on macOS).
    *   Navigate to the ODPI-C library directory within the project:
        ```bash
        cd internal/lib/odpi
        ```
    *   **Important Configuration:** Open the `Makefile` located in this directory (`internal/lib/odpi/Makefile`) with a text editor.
        *   Locate the `INSTANT_CLIENT_DIR` variable.
        *   **You MUST update its value** to point to the `sdk/lib/msvc` (for Windows) or `sdk/lib` (for Linux/macOS) subdirectory within your Oracle Instant Client installation path.
            *   Example for Windows: `INSTANT_CLIENT_DIR = C:/oracle_inst/instantclient_23_7/sdk/lib/msvc`
            *   Example for Linux/macOS: `INSTANT_CLIENT_DIR = /opt/oracle/instantclient_23_7/sdk/lib` (Adjust path as needed)
    *   Build the shared library using `make`. If you are on Windows using MinGW, you might need to use `mingw32-make` if `make` is not aliased.
        ```bash
        make 
        ```
        Or for MinGW if `make` is not found:
        ```bash
        mingw32-make
        ```
        This command will compile the necessary C source files (including those for ODPI-C and any custom helpers like `dpi_helpers.c` if configured in the Makefile) and create the shared library (e.g., `lib/odpi.dll`) in the `internal/lib/odpi/lib` directory.

3.  **Build and Run the Go Application:**
    *   Navigate back to the project root directory:
        ```bash
        cd ../../../ 
        ``` 
        (This command assumes you are in `internal/lib/odpi`)
    *   Build the Go application:
        ```bash
        go build
        ```
        This will create an executable file (e.g., `OmniView.exe` on Windows or `OmniView` on Linux/macOS) in the project root.
    *   Run the application:
        *   Using `go run`:
            ```bash
            go run main.go
            ```
        *   Or by executing the compiled binary:
            ```bash
            ./OmniView  # On Linux/macOS
            ```
            ```powershell
            .\OmniView.exe # On Windows PowerShell
            ```
            ```cmd
            OmniView.exe   # On Windows Command Prompt
            ```

## Usage



## Project Structure

```
.
├── main.go         # Application entry point
├── go.mod          # Go module definition
└── README.md       # Project documentation
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.