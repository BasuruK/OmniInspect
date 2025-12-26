import os
import sys
import shutil
import zipfile
import urllib.request
import platform
import subprocess
import argparse

ODPI_VERSION = "5.6.4"
ODPI_DIR = f"odpi-{ODPI_VERSION}"
ODPI_ZIP = "odpi.zip"
ODPI_URL = f"https://github.com/oracle/odpi/archive/refs/tags/v{ODPI_VERSION}.zip"

# Get the script's directory (ai_agents folder)
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
# Go up one level to project root
PROJECT_ROOT = os.path.dirname(SCRIPT_DIR)

ODPI_PATH = os.path.join(PROJECT_ROOT, ODPI_DIR)
ODPI_ZIP_PATH = os.path.join(PROJECT_ROOT, ODPI_ZIP)
INCLUDE_SRC = os.path.join(ODPI_PATH, "include")
SRC_SRC = os.path.join(ODPI_PATH, "src")
DEST_INCLUDE = os.path.join(PROJECT_ROOT, "third_party", "odpi", "include")
DEST_SRC = os.path.join(PROJECT_ROOT, "third_party", "odpi", "src")

# Detect platform
SYSTEM = platform.system()
MACHINE = platform.machine()
IS_WINDOWS = SYSTEM == "Windows"
IS_MACOS = SYSTEM == "Darwin"
IS_APPLE_SILICON = IS_MACOS and MACHINE == "arm64"


def log(msg):
    """Logs a message to the console."""
    print(msg)

def detect_platform():
    """Detects and displays platform information."""
    log("Platform Information:")
    log(f"      OS: {SYSTEM}")
    log(f"      Architecture: {MACHINE}")
    if IS_APPLE_SILICON:
        log(f"      [OK] Detected Apple Silicon (M1/M2/M3/M4)")
    elif IS_WINDOWS:
        log(f"      [OK] Detected Windows")
    log(f"      Project Root: {PROJECT_ROOT}")

def download_odpi():
    """Downloads and extracts the ODPI-C library if it doesn't exist."""
    if os.path.exists(ODPI_PATH):
        log(f"[INFO] {ODPI_DIR} already exists, skipping download and extraction")
        return
    if os.path.exists(ODPI_ZIP_PATH):
        log(f"[INFO] {ODPI_ZIP} already exists, skipping download")
    else:
        log(f"Downloading ODPI-C v{ODPI_VERSION}...")
        try:
            urllib.request.urlretrieve(ODPI_URL, ODPI_ZIP_PATH)
            log("       [OK] Download successful")
        except Exception as e:
            log(f"[ERROR] Download failed: {e}")
            exit(1)
    log("Extracting ODPI-C library...")
    try:
        with zipfile.ZipFile(ODPI_ZIP_PATH, 'r') as zip_ref:
            zip_ref.extractall(PROJECT_ROOT)
        log("       [OK] Extraction completed successfully")
    except Exception as e:
        log(f"[ERROR] Extraction failed: {e}")
        exit(1)

def verify_dirs():
    """Verifies the presence of include/ and src/ directories after extraction."""
    log("Verifying extracted directories...")
    if os.path.isdir(INCLUDE_SRC) and os.path.isdir(SRC_SRC):
        log("       [OK] Verification successful - both include/ and src/ directories exist")
    else:
        log("[ERROR] Verification failed - required directories missing")
        exit(1)

def copy_headers():
    """Copies header files from the extracted ODPI-C include directory to the project's internal include directory."""
    log("Copying header files...")
    os.makedirs(DEST_INCLUDE, exist_ok=True)
    header_files = [f for f in os.listdir(INCLUDE_SRC) if f.endswith('.h')]
    for f in header_files:
        shutil.copy2(os.path.join(INCLUDE_SRC, f), DEST_INCLUDE)
    if header_files:
        log(f"      [OK] Header files copied successfully ({len(header_files)})")
    else:
        log("[ERROR] No header files found to copy")
        exit(1)

def copy_sources():
    """Copies source files from the extracted ODPI-C src directory to the project's internal src directory."""
    log("Copying source files...")
    os.makedirs(DEST_SRC, exist_ok=True)
    src_files = os.listdir(SRC_SRC)
    for f in src_files:
        shutil.copy2(os.path.join(SRC_SRC, f), DEST_SRC)
    if src_files:
        log(f"      [OK] Source files copied successfully ({len(src_files)})")
    else:
        log("[ERROR] No source files found to copy")
        exit(1)

def copy_c_from_include():
    """Copies custom .c files from the internal include directory to the internal src directory."""
    log("Copying custom .c files from include to src...")
    c_files = [f for f in os.listdir(DEST_INCLUDE) if f.endswith('.c')]
    if c_files:
        for f in c_files:
            shutil.copy2(os.path.join(DEST_INCLUDE, f), DEST_SRC)
        log(f"      [OK] Copied {len(c_files)} .c files to src directory")
    else:
        log("[ERROR] No .c files found in include directory")

def cleanup():
    """Cleans up temporary files and directories created during the setup process."""
    log("Cleaning up temporary files...")
    removed = 0
    errors = []
    for path, typ in [(ODPI_ZIP_PATH, "file"), (ODPI_PATH, "directory")]:
        try:
            if os.path.exists(path):
                log(f"      [CLEAN] Removing {typ}: {path}")
                if typ == "directory":
                    # Handle Windows readonly files
                    if IS_WINDOWS:
                        def handle_remove_readonly(func, path, exc):
                            os.chmod(path, 0o777)
                            func(path)
                        shutil.rmtree(path, onerror=handle_remove_readonly)
                    else:
                        shutil.rmtree(path)
                else:
                    os.remove(path)
                removed += 1
            else:
                log(f"[INFO] {typ} {path} does not exist, skipping...")
        except Exception as e:
            errors.append(f"Failed to remove {typ} '{path}': {e}")
    if errors:
        log("[WARN] Encountered errors during cleanup:")
        for err in errors:
            log(f"   {err}")
        log("[ERROR] Please manually delete the problematic files")
    if removed:
        log(f"      [OK] Cleanup successful - Removed {removed} item(s)")
    else:
        log("[INFO] No items needed cleanup")

def run_make():
    """Navigates to project root and runs make, then make clean."""
    odpi_build_dir = PROJECT_ROOT
    
    if not os.path.exists(odpi_build_dir):
        log(f"[ERROR] Build directory does not exist: {odpi_build_dir}")
        return False
    
    log("")
    log("=" * 60)
    log("Building ODPI-C library...")
    log("=" * 60)
    
    try:
        # Run make
        log(f"Running 'make' in {odpi_build_dir}...")
        result = subprocess.run(
            ["make"],
            cwd=odpi_build_dir,
            check=True,
            capture_output=True,
            text=True
        )
        log("       [OK] Build completed successfully")
        if result.stdout:
            log(f"\nBuild output:\n{result.stdout}")
        
        # Run make clean
        log("\nRunning 'make clean'...")
        result = subprocess.run(
            ["make", "clean"],
            cwd=odpi_build_dir,
            check=True,
            capture_output=True,
            text=True
        )
        log("       [OK] Cleanup completed successfully")
        if result.stdout:
            log(f"\nCleanup output:\n{result.stdout}")
        
        return True
    except subprocess.CalledProcessError as e:
        log(f"[ERROR] Make command failed with exit code {e.returncode}")
        if e.stdout:
            log(f"stdout: {e.stdout}")
        if e.stderr:
            log(f"stderr: {e.stderr}")
        return False
    except FileNotFoundError:
        log("[ERROR] 'make' command not found. Please ensure make is installed and in your PATH.")
        return False

def main():
    """Main function to orchestrate the ODPI-C setup process."""
    # Parse command line arguments
    parser = argparse.ArgumentParser(description="ODPI-C Setup Script")
    parser.add_argument("--make", action="store_true", help="Run make and make clean after setup")
    args = parser.parse_args()
    
    log("=" * 60)
    log("ODPI-C Setup Script")
    log("=" * 60)
    detect_platform()
    log("")
    
    download_odpi()
    verify_dirs()
    copy_headers()
    copy_sources()
    copy_c_from_include()
    cleanup()
    
    log("")
    log("=" * 60)
    log("[SUCCESS] ODPI-C setup completed successfully!")
    log("=" * 60)
    
    # Run make if --make flag is provided
    if args.make:
        if run_make():
            log("")
            log("=" * 60)
            log("[SUCCESS] Build and cleanup completed successfully!")
            log("=" * 60)
        else:
            log("")
            log("=" * 60)
            log("[WARN] Setup completed but build failed")
            log("=" * 60)
            sys.exit(1)
    else:
        if IS_APPLE_SILICON:
            log("\n[NOTE] Next steps for Apple Silicon:")
            log("   1. Install Oracle Instant Client ARM64:")
            log("      Download from: https://www.oracle.com/database/technologies/instant-client/macos-arm64-downloads.html")
            log("   2. Extract to: /opt/oracle/instantclient_23_7")
            log("   3. Build ODPI-C:")
            log("      cd third_party/odpi && make")
            log("\n   Or run this script with --make flag to build automatically:")
            log("      python ai_agents/setup_odpi.py --make")
        elif IS_WINDOWS:
            log("\n[NOTE] Next steps for Windows:")
            log("   1. Install Oracle Instant Client:")
            log("      Download from: https://www.oracle.com/database/technologies/instant-client/winx64-64-downloads.html")
            log("   2. Extract to: C:\\oracle_inst\\instantclient_23_7")
            log("   3. Build ODPI-C:")
            log("      cd third_party\\odpi && make")
            log("\n   Or run this script with --make flag to build automatically:")
            log("      python ai_agents\\setup_odpi.py --make")

if __name__ == "__main__":
    main()