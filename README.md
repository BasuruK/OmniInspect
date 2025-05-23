# 👋 Welcome to OmniView!

OmniView is your friendly Go-based application designed for seamless background tracing activities. We're here to make your tracing tasks easier and more efficient! 🕵️‍♂️💻

## 🚀 Getting Started

Let's get you set up and running with OmniView! This guide will walk you through the necessary steps.

### ✅ Prerequisites

Before you dive in, please make sure you have the following tools installed and ready:

*   **Go (Version 1.24+):** 🐹
    *   You'll need Go version 1.24 or newer (as noted in our `go.mod` file).
    *   A standard Go development environment is perfect. Ensure `GOROOT` and `GOPATH` are set up, and Go's `bin` directory is in your system `PATH`.
*   **C Compiler (for ODPI-C):** ⚙️ This is needed to help OmniView talk to Oracle databases.
    *   **Windows Users:** MinGW-w64 is your go-to (you can get it via MSYS2 or directly from the MinGW-w64 site). Make sure `gcc.exe` and `make.exe` (or `mingw32-make.exe`) are accessible from your command line (added to `PATH`).
    *   **Linux/macOS Users:** GCC and Make are usually already there. If not, you can typically install them with package managers (e.g., `build-essential` on Debian/Ubuntu, or Xcode Command Line Tools on macOS).
*   **Oracle Instant Client:** 📦 This is key for connecting to Oracle databases.
    *   **Version:** We've set up our system for Instant Client version 23.7 (e.g., `instantclient_23_7`). You can grab this or a newer compatible version.
    *   **Download Link:** Head over to the [Oracle Instant Client Downloads page](https://www.oracle.com/database/technologies/instant-client/downloads.html).
    *   **What to Download (ZIP files):**
        *   **Basic Package** (or Basic Light Package): This has all the essential client libraries.
        *   **SDK Package**: This includes important files (headers and libraries) needed for compiling parts of OmniView like ODPI-C.
    *   **Installation Steps (Super Important! ✨):**
        1.  **Create a Home for Instant Client:** Make a new directory for your Instant Client files (e.g., `C:\oracle_inst\instantclient_23_7` on Windows, or `/opt/oracle/instantclient_23_7` on Linux/macOS).
        2.  **Unzip the Goodies:** Extract both the Basic (or Basic Light) and SDK packages into the directory you just created. You should see subdirectories like `sdk/include` and `sdk/lib/msvc` (Windows) or `sdk/lib` (Linux/macOS).
        3.  **Tell Your System Where to Find It (Environment Variable):** Add the main Instant Client directory (the one you created in step 1, e.g., `C:\oracle_inst\instantclient_23_7`) to your system's `PATH`. This helps your computer find the Oracle client files when OmniView runs.
            *   **Linux/macOS Tip:** You might also need to set up `LD_LIBRARY_PATH` or use `ldconfig` if you've placed the libraries in a non-standard spot.

### 🛠️ Installation Steps

Follow these steps to get OmniView installed:

1.  **Navigate to Your Project Folder:** 📂
    Open your terminal or command prompt and go to the root directory where you have the OmniView project files.
    ```bash
    cd path/to/OmniView
    ```

2.  **Build the ODPI-C Magic ✨ (Shared Library):**
    OmniView uses a C library called ODPI-C to connect to Oracle Databases. We need to compile this into a shared library (`odpi.dll` on Windows, `odpi.so` on Linux, `odpi.dylib` on macOS).
    *   Go to the ODPI-C library directory:
        ```bash
        cd internal/lib/odpi
        ```
    *   **Super Important Configuration! ⚙️**
        *   Open the `Makefile` in this directory (`internal/lib/odpi/Makefile`) with your favorite text editor.
        *   Find the line that says `INSTANT_CLIENT_DIR`.
        *   **You MUST change its value** to point to the `sdk/lib/msvc` (for Windows) or `sdk/lib` (for Linux/macOS) folder inside your Oracle Instant Client installation.
            *   Example (Windows): `INSTANT_CLIENT_DIR = C:/oracle_inst/instantclient_23_7/sdk/lib/msvc`
            *   Example (Linux/macOS): `INSTANT_CLIENT_DIR = /opt/oracle/instantclient_23_7/sdk/lib` (Remember to use your actual path!)
    *   Now, build the library using `make`. If you're on Windows with MinGW, you might need to use `mingw32-make`.
        ```bash
        make
        ```
        Or for MinGW, if `make` isn't recognized:
        ```bash
        mingw32-make
        ```
        This will create the shared library (e.g., `lib/odpi.dll`) in the `internal/lib/odpi/lib` directory.

3.  **Build and Run OmniView! 🏃‍♂️**
    *   Head back to the main project root directory:
        ```bash
        cd ../../../
        ```
        (This assumes you're currently in `internal/lib/odpi`)
    *   Build the Go application:
        ```bash
        go build
        ```
        This creates your OmniView executable (like `OmniView.exe` on Windows or `OmniView` on Linux/macOS) in the project root.
    *   Run it!
        *   Quick run with `go run`:
            ```bash
            go run main.go
            ```
        *   Or run the compiled program:
            ```bash
            ./OmniView  # On Linux/macOS
            ```
            ```powershell
            .\OmniView.exe # On Windows PowerShell
            ```
            ```cmd
            OmniView.exe   # On Windows Command Prompt
            ```

## 📖 Usage

Details on how to use OmniView will be added here soon! Stay tuned! 🚀

## 📂 Project Structure

Here's a peek at how our project is organized: 🏗️
```
.
├── main.go         # 🎯 Application entry point
├── go.mod          # 📦 Go module definition
└── README.md       # 📚 Project documentation (You are here!)
```

## 📜 License

This project is proprietary and all rights are reserved. 🔒
Please see the `LICENSE` file in this repository for full details.

Thank you for choosing OmniView! We hope you find it useful. If you have any questions or feedback, please let us know (though we don't have a formal support channel yet!). 😊