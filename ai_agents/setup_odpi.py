import os, shutil, zipfile, urllib.request

ODPI_VERSION = "5.5.1"
ODPI_DIR = f"odpi-{ODPI_VERSION}"
ODPI_ZIP = "odpi.zip"
ODPI_URL = f"https://github.com/oracle/odpi/archive/refs/tags/v{ODPI_VERSION}.zip"

PWD = os.getcwd()
ODPI_PATH = os.path.join(PWD, ODPI_DIR)
ODPI_ZIP_PATH = os.path.join(PWD, ODPI_ZIP)
INCLUDE_SRC = os.path.join(ODPI_PATH, "include")
SRC_SRC = os.path.join(ODPI_PATH, "src")
DEST_INCLUDE = os.path.join(PWD, "internal", "lib", "odpi", "include")
DEST_SRC = os.path.join(PWD, "internal", "lib", "odpi", "src")


def log(msg):
    """Logs a message to the console."""
    print(msg)

def download_odpi():
    """Downloads and extracts the ODPI-C library if it doesn't exist."""
    if os.path.exists(ODPI_PATH):
        log(f"ℹ️ {ODPI_DIR} already exists, skipping download and extraction")
        return
    if os.path.exists(ODPI_ZIP_PATH):
        log(f"ℹ️ {ODPI_ZIP} already exists, skipping download")
    else:
        log(f"Downloading ODPI-C v{ODPI_VERSION}...")
        try:
            urllib.request.urlretrieve(ODPI_URL, ODPI_ZIP_PATH)
            log("       ✅ Download successful")
        except Exception as e:
            log(f"❌ Download failed: {e}")
            exit(1)
    log("Extracting ODPI-C library...")
    try:
        with zipfile.ZipFile(ODPI_ZIP_PATH, 'r') as zip_ref:
            zip_ref.extractall(PWD)
        log("       ✅ Extraction completed successfully")
    except Exception as e:
        log(f"❌ Extraction failed: {e}")
        exit(1)

def verify_dirs():
    """Verifies the presence of include/ and src/ directories after extraction."""
    log("Verifying extracted directories...")
    if os.path.isdir(INCLUDE_SRC) and os.path.isdir(SRC_SRC):
        log("       ✅ Verification successful - both include/ and src/ directories exist")
    else:
        log("❌ Verification failed - required directories missing")
        exit(1)

def copy_headers():
    """Copies header files from the extracted ODPI-C include directory to the project's internal include directory."""
    log("Copying header files...")
    os.makedirs(DEST_INCLUDE, exist_ok=True)
    header_files = [f for f in os.listdir(INCLUDE_SRC) if f.endswith('.h')]
    for f in header_files:
        shutil.copy2(os.path.join(INCLUDE_SRC, f), DEST_INCLUDE)
    if header_files:
        log(f"      ✅ Header files copied successfully ({len(header_files)})")
    else:
        log("❌ No header files found to copy")
        exit(1)

def copy_sources():
    """Copies source files from the extracted ODPI-C src directory to the project's internal src directory."""
    log("Copying source files...")
    os.makedirs(DEST_SRC, exist_ok=True)
    src_files = os.listdir(SRC_SRC)
    for f in src_files:
        shutil.copy2(os.path.join(SRC_SRC, f), DEST_SRC)
    if src_files:
        log(f"      ✅ Source files copied successfully ({len(src_files)})")
    else:
        log("❌ No source files found to copy")
        exit(1)

def copy_c_from_include():
    """Copies custom .c files from the internal include directory to the internal src directory."""
    log("Copying custom .c files from include to src...")
    c_files = [f for f in os.listdir(DEST_INCLUDE) if f.endswith('.c')]
    if c_files:
        for f in c_files:
            shutil.copy2(os.path.join(DEST_INCLUDE, f), DEST_SRC)
        log(f"      ✅ Copied {len(c_files)} .c files to src directory")
    else:
        log("❌ No .c files found in include directory")

def cleanup():
    """Cleans up temporary files and directories created during the setup process."""
    log("Cleaning up temporary files...")
    removed = 0
    errors = []
    for path, typ in [(ODPI_ZIP_PATH, "file"), (ODPI_PATH, "directory")]:
        try:
            if os.path.exists(path):
                log(f"      🪲 Removing {typ}: {path}")
                if typ == "directory":
                    shutil.rmtree(path)
                else:
                    os.remove(path)
                removed += 1
            else:
                log(f"ℹ️ {typ} {path} does not exist, skipping...")
        except Exception as e:
            errors.append(f"Failed to remove {typ} '{path}': {e}")
    if errors:
        log("⚠️ Encountered errors during cleanup:")
        for err in errors:
            log(f"   {err}")
        log("❌ Please manually delete the problematic files")
    if removed:
        log(f"      ✅ Cleanup successful - Removed {removed} item(s)")
    else:
        log("ℹ️ No items needed cleanup")

def main():
    """Main function to orchestrate the ODPI-C setup process."""
    download_odpi()
    verify_dirs()
    copy_headers()
    copy_sources()
    copy_c_from_include()
    cleanup()
    log("ODPI-C setup completed successfully \U0001F973\U0001F389")

if __name__ == "__main__":
    main()