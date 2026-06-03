package updater

import (
	"OmniView/internal/adapter/logger"
	"OmniView/internal/core/domain"
	"archive/tar"
	"archive/zip"
	"compress/gzip"

	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// GitHubRepo is the public GitHub repository to check for releases.
const GitHubRepo = "BasuruK/OmniInspect"

// gitHubAPIBase is the base URL for the GitHub API.
const gitHubAPIBase = "https://api.github.com"

const maxArchiveEntrySize int64 = 100 * 1024 * 1024
const maxExtractedArchiveSize int64 = 500 * 1024 * 1024

// releaseAsset represents a single downloadable file attached to a GitHub release.
type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// githubRelease represents the JSON response from the GitHub Releases API.
type githubRelease struct {
	TagName     string         `json:"tag_name"`
	Body        string         `json:"body"`
	PublishedAt string         `json:"published_at"`
	Assets      []releaseAsset `json:"assets"`
}

// UpdateInfo contains information about an available update.
// Returned by CheckForUpdate when an update is available.
type UpdateInfo struct {
	Available      bool
	CurrentVersion string
	NewVersion     string
	ReleaseNotes   string
	DownloadURL    string
	PublishedAt    string
	Assets         []releaseAsset
}

// CheckForUpdate checks the latest GitHub release and returns update information.
// currentVersion is the version string embedded in the binary at build time (e.g. "v0.2.0").
// Returns (*UpdateInfo, nil) when an update is available.
// Returns (nil, nil) when no update is needed or in development mode.
// Returns (nil, error) on failure.
func CheckForUpdate(ctx context.Context, currentVersion string) (*UpdateInfo, error) {
	// Skip update check for development builds
	if currentVersion == "dev" || currentVersion == "" {
		return nil, nil
	}

	// Fetch the latest release from GitHub
	release, err := fetchLatestRelease(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest release: %w", err)
	}

	// Compare versions
	if !isNewer(release.TagName, currentVersion) {
		return nil, nil
	}

	// Find the correct asset for this platform
	assetName := expectedAssetName(release.TagName)
	var downloadURL string
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return nil, domain.ErrNoMatchingReleaseAsset
	}

	return &UpdateInfo{
		Available:      true,
		CurrentVersion: currentVersion,
		NewVersion:     release.TagName,
		ReleaseNotes:   release.Body,
		DownloadURL:    downloadURL,
		PublishedAt:    release.PublishedAt,
		Assets:         release.Assets,
	}, nil
}

// DownloadAndApply downloads the update, verifies the checksum, extracts it,
// and restarts the application. progressFn is called at each stage with
// descriptive strings: "Downloading...", "Verifying checksum...", "Extracting...",
// "Restarting...". The caller is responsible for user interaction (e.g., prompting
// for confirmation before calling this function).
func DownloadAndApply(ctx context.Context, info *UpdateInfo, progressFn func(stage string)) error {
	if info == nil || !info.Available {
		return domain.ErrNoUpdateInfo
	}

	progress := func(stage string) {
		if progressFn != nil {
			progressFn(stage)
		}
	}

	progress("Downloading...")

	// Download to a temporary file
	tmpFile, err := downloadToTemp(ctx, info.DownloadURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer os.Remove(tmpFile) // Clean up the temp archive

	// Build release object from UpdateInfo for checksum verification and release notes
	release := &githubRelease{
		TagName:     info.NewVersion,
		Body:        info.ReleaseNotes,
		PublishedAt: info.PublishedAt,
		Assets:      info.Assets,
	}

	progress("Verifying checksum...")
	if _, err := verifyChecksum(ctx, tmpFile, release, expectedAssetName(info.NewVersion)); err != nil {
		return fmt.Errorf("checksum verification failed: %w", err)
	}
	// verified == false with err == nil means no checksum file was found - skip verification
	// verified == true means checksum was verified successfully

	// Resolve our own executable path
	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}
	selfPath, err = filepath.EvalSymlinks(selfPath)
	if err != nil {
		return fmt.Errorf("cannot resolve executable symlinks: %w", err)
	}
	selfDir := filepath.Dir(selfPath)

	progress("Extracting...")

	// Extract the archive into the same directory as the running binary
	if err := extractArchive(tmpFile, selfDir, selfPath); err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}

	// Write RELEASE_NOTES.md in the same directory as the binary. A failure here
	// is non-fatal — the binary has already been swapped in and the user can
	// still read release notes from GitHub — but it is logged for diagnosis.
	if err := writeReleaseNotes(selfDir, release); err != nil {
		logger.Error("writeReleaseNotes failed",
			"release", release.TagName,
			"dir", selfDir,
			"err", err,
		)
	}

	// Clean up temp archive before restart (defers won't run after os.Exit)
	os.Remove(tmpFile)

	progress("Restarting...")

	// Restart the application
	return restartSelf(selfPath)
}

// checkAndUpdate checks the latest GitHub release and prompts the user to update.
// currentVersion is the version string embedded in the binary at build time (e.g. "v0.2.0").
// Returns nil if no update is needed or the user declines. Returns an error on failure.
//
// Deprecated: Use CheckForUpdate followed by DownloadAndApply instead.
// This function is kept for backward compatibility with non-TUI entry points and
// the CLI updater. It uses fmt.Scanln for interactive prompting, which is not
// safe to invoke from within a Bubble Tea program (the TUI owns stdin/stdout).
// Unexported because all in-process callers should go through the TUI's
// UpdaterService, which uses CheckForUpdate + DownloadAndApply.
func checkAndUpdate(currentVersion string) error {
	info, err := CheckForUpdate(context.Background(), currentVersion)
	if err != nil {
		fmt.Printf("[updater] Could not check for updates: %v\n", err)
		return nil // Non-fatal: don't block the application from starting
	}

	if info == nil {
		return nil // No update available
	}

	// Prompt the user
	fmt.Printf("[updater] Update %s available (current: %s).\n", info.NewVersion, info.CurrentVersion)
	fmt.Print("[updater] Update now? [Y/n]: ")

	var answer string
	fmt.Scanln(&answer)
	answer = strings.TrimSpace(strings.ToLower(answer))

	if answer != "" && answer != "y" && answer != "yes" {
		fmt.Println("[updater] Update skipped.")
		return nil
	}

	return DownloadAndApply(context.Background(), info, func(stage string) {
		fmt.Printf("[updater] %s\n", stage)
	})
}

// CleanupOldBinary removes the ".old" leftovers from a previous update, if they exist.
// This includes the executable itself and any shared libraries (.dll / .dylib) that were
// renamed during extraction because they were locked by the running process.
// Call this at the very top of main().
func CleanupOldBinary() {
	selfPath, err := os.Executable()
	if err != nil {
		return
	}
	if resolvedPath, err := filepath.EvalSymlinks(selfPath); err == nil {
		selfPath = resolvedPath
	}
	selfDir := filepath.Dir(selfPath)
	oldPath := selfPath + ".old"
	if _, err := os.Stat(oldPath); err == nil {
		os.Remove(oldPath)
	}

	// Clean up any old shared libraries (.dll on Windows, .dylib on macOS)
	var patterns []string
	if runtime.GOOS == "windows" {
		patterns = []string{"*.dll.old"}
	} else {
		patterns = []string{"*.dylib.old"}
	}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(selfDir, pattern))
		if err != nil {
			continue
		}
		for _, match := range matches {
			os.Remove(match)
		}
	}
}

// --------------------------------------------------------------------------
// Internal helpers
// --------------------------------------------------------------------------

// fetchLatestRelease calls the GitHub API and returns the latest stable release.
func fetchLatestRelease(ctx context.Context) (*githubRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", gitHubAPIBase, GitHubRepo)

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "OmniView-Updater")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release JSON: %w", err)
	}
	return &release, nil
}

// isNewer returns true if remoteTag is a higher version than localVersion.
// Both are expected in the form "vX.Y.Z" or "X.Y.Z".
func isNewer(remoteTag, localVersion string) bool {
	remote := parseVersion(remoteTag)
	local := parseVersion(localVersion)

	for i := 0; i < 3; i++ {
		if remote[i] > local[i] {
			return true
		}
		if remote[i] < local[i] {
			return false
		}
	}
	return false // Same version
}

// parseVersion turns "v1.2.3" or "1.2.3" into [3]int{1, 2, 3}.
// Any non-numeric suffix (e.g. "-beta") is stripped. Unparsable segments become 0.
func parseVersion(v string) [3]int {
	v = strings.TrimPrefix(v, "v")

	// Strip pre-release suffix: "1.2.3-beta" → "1.2.3"
	if idx := strings.IndexAny(v, "-+"); idx != -1 {
		v = v[:idx]
	}

	parts := strings.SplitN(v, ".", 3)
	var result [3]int
	for i := 0; i < len(parts) && i < 3; i++ {
		n, err := strconv.Atoi(parts[i])
		if err == nil {
			result[i] = n
		}
	}
	return result
}

// expectedAssetName returns the release asset filename expected for this platform.
// Convention: omniview-{os}-{arch}-{tag}.zip  (Windows)
//
//	omniview-{os}-{arch}-{tag}.tar.gz (macOS)
func expectedAssetName(tag string) string {
	osName := runtime.GOOS // "windows" or "darwin"
	arch := runtime.GOARCH // "amd64" or "arm64"

	if osName == "windows" {
		return fmt.Sprintf("omniview-%s-%s-%s.zip", osName, arch, tag)
	}
	return fmt.Sprintf("omniview-%s-%s-%s.tar.gz", osName, arch, tag)
}

// downloadToTemp downloads the given URL to a temporary file and returns its path.
func downloadToTemp(ctx context.Context, url string) (string, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "omniview-update-*")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}

	return tmpFile.Name(), nil
}

// verifyChecksum verifies the SHA256 checksum of the downloaded file.
// It looks for a checksum asset in the release (e.g., assetName+".sha256" or "checksums.txt")
// and compares the computed hash against the expected value.
// Returns (true, nil) when checksum is verified, (false, nil) when no checksum file exists,
// and (false, err) for verification failures.
func verifyChecksum(ctx context.Context, tmpFile string, release *githubRelease, assetName string) (bool, error) {
	// Try to find a checksum file in the release assets
	// Common patterns: <asset>.sha256, checksums.txt, SHA256SUMS, etc.
	var checksumURL string
	var checksumAssetName string

	// Strategy 1: Look for assetName + ".sha256"
	expectedChecksumName := assetName + ".sha256"
	for _, asset := range release.Assets {
		if asset.Name == expectedChecksumName {
			checksumURL = asset.BrowserDownloadURL
			checksumAssetName = asset.Name
			break
		}
	}

	// Strategy 2: Look for common checksum file names
	if checksumURL == "" {
		checksumFileNames := []string{"checksums.txt", "SHA256SUMS", "checksums.sha256"}
		for _, name := range checksumFileNames {
			for _, asset := range release.Assets {
				if asset.Name == name {
					checksumURL = asset.BrowserDownloadURL
					checksumAssetName = asset.Name
					break
				}
			}
			if checksumURL != "" {
				break
			}
		}
	}

	if checksumURL == "" {
		return false, nil
	}

	// Download the checksum file
	checksumData, err := downloadChecksumFile(ctx, checksumURL)
	if err != nil {
		return false, fmt.Errorf("failed to download checksum file %s: %w", checksumAssetName, err)
	}

	// Authenticate the checksum file with a detached signature when release signing
	// is provisioned. This is fail-closed: a configured key requires a valid signature.
	if _, err := verifyChecksumFileSignature(ctx, release, checksumAssetName, checksumData); err != nil {
		return false, fmt.Errorf("signature verification failed: %w", err)
	}

	// Parse the expected checksum from the downloaded data
	expectedChecksum, err := parseChecksum(checksumData, assetName)
	if err != nil {
		return false, fmt.Errorf("failed to parse checksum for %s: %w", assetName, err)
	}

	// Compute the SHA256 of the downloaded archive
	computedChecksum, err := computeSHA256(tmpFile)
	if err != nil {
		return false, fmt.Errorf("failed to compute checksum of downloaded file: %w", err)
	}

	// Compare checksums (case-insensitive)
	if !strings.EqualFold(computedChecksum, expectedChecksum) {
		return false, fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, computedChecksum)
	}

	return true, nil
}

// downloadChecksumFile downloads the checksum file and returns its content as a string.
// The context is used to cancel the in-flight HTTP request; checksum files are tiny
// (a few hundred bytes), so no separate size cap is required.
func downloadChecksumFile(ctx context.Context, url string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// parseChecksum extracts the checksum for the given asset from checksum file content.
// Supports formats like:
//   - "abc123..."  (single checksum, raw)
//   - "abc123... *filename" (sha256sum format)
//   - "abc123...  filename" (sha256sum format with multiple spaces)
func parseChecksum(checksumData, assetName string) (string, error) {
	lines := strings.Split(checksumData, "\n")

	// If there's only one line and it looks like a raw checksum, use it
	if len(lines) == 1 || (len(lines) == 2 && strings.TrimSpace(lines[1]) == "") {
		checksum := strings.TrimSpace(lines[0])
		// Check if it's a valid hex string (64 chars for SHA256)
		if len(checksum) == 64 && isValidHex(checksum) {
			return checksum, nil
		}
	}

	// Otherwise, parse as sha256sum format: "<checksum> <filename>"
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split on whitespace
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		checksum := fields[0]
		// The filename might be prefixed with * (binary mode indicator)
		filename := strings.TrimPrefix(fields[len(fields)-1], "*")

		if filename == assetName {
			if len(checksum) == 64 && isValidHex(checksum) {
				return checksum, nil
			}
		}
	}

	return "", fmt.Errorf("checksum for %s not found in checksum file", assetName)
}

// isValidHex checks if a string contains only valid hexadecimal characters.
func isValidHex(s string) bool {
	_, err := hex.DecodeString(s)
	return err == nil
}

// computeSHA256 computes the SHA256 hash of the file at the given path.
func computeSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// extractArchive extracts a .zip or .tar.gz archive into destDir.
// selfPath is the path of the currently running binary — it gets renamed to .old
// before being overwritten.
func extractArchive(archivePath, destDir, selfPath string) error {
	if runtime.GOOS == "windows" {
		return extractZip(archivePath, destDir, selfPath)
	}
	return extractTarGz(archivePath, destDir, selfPath)
}

// extractZip extracts a .zip archive. Files that may be locked by the running
// process (the executable itself and any .dll shared libraries) are renamed to
// .old before the new version is written. This works on Windows because the OS
// allows renaming a loaded DLL/EXE — it only prevents deletion or overwriting.
func extractZip(archivePath, destDir, selfPath string) error {
	return extractZipWithLimit(archivePath, destDir, selfPath, maxExtractedArchiveSize)
}

// extractZipWithLimit is the testable core of extractZip, accepting an explicit
// maxTotalSize cap so unit tests can exercise the size-limit path without creating
// multi-megabyte archives.
func extractZipWithLimit(archivePath, destDir, selfPath string, maxTotalSize int64) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}
	defer r.Close()

	selfName := filepath.Base(selfPath)
	totalSize := int64(0)

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}

		if !f.FileInfo().Mode().IsRegular() {
			continue
		}

		name, ok := safeArchiveFileName(f.Name)
		if !ok {
			continue
		}

		if f.UncompressedSize64 > uint64(maxArchiveEntrySize) {
			return fmt.Errorf("archive entry %s exceeds maximum size limit of %d bytes", name, maxArchiveEntrySize)
		}

		entrySize := int64(f.UncompressedSize64)
		if totalSize > maxTotalSize-entrySize {
			return fmt.Errorf("archive exceeds maximum extracted size limit of %d bytes", maxTotalSize)
		}
		totalSize += entrySize

		destPath, err := archiveDestPath(destDir, name)
		if err != nil {
			return err
		}

		if err := writeFileFromReader(f.Open, destPath, f.FileInfo().Mode(), selfName); err != nil {
			return err
		}
	}
	return nil
}

// extractTarGz extracts a .tar.gz archive. Files that may be locked by the
// running process (the executable itself and any .dylib shared libraries) are
// renamed to .old before the new version is written.
func extractTarGz(archivePath, destDir, selfPath string) error {
	return extractTarGzWithLimit(archivePath, destDir, selfPath, maxExtractedArchiveSize)
}

// extractTarGzWithLimit is the testable core of extractTarGz, accepting an explicit
// maxTotalSize cap so unit tests can exercise the size-limit path without creating
// multi-megabyte archives.
func extractTarGzWithLimit(archivePath, destDir, selfPath string, maxTotalSize int64) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gz.Close()

	limitedGz := &io.LimitedReader{R: gz, N: maxTotalSize + 1}
	tr := tar.NewReader(limitedGz)
	selfName := filepath.Base(selfPath)
	totalSize := int64(0)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			if limitedGz.N == 0 {
				return fmt.Errorf("archive exceeds maximum extracted size limit of %d bytes", maxTotalSize)
			}
			return fmt.Errorf("tar read error: %w", err)
		}

		if header.Typeflag != tar.TypeReg {
			continue // Skip directories and special files
		}

		name, ok := safeArchiveFileName(header.Name)
		if !ok {
			continue
		}

		if header.Size > maxArchiveEntrySize {
			return fmt.Errorf("archive entry %s exceeds maximum size limit of %d bytes", name, maxArchiveEntrySize)
		}
		if totalSize > maxTotalSize-header.Size {
			return fmt.Errorf("archive exceeds maximum extracted size limit of %d bytes", maxTotalSize)
		}
		totalSize += header.Size

		destPath, err := archiveDestPath(destDir, name)
		if err != nil {
			return err
		}

		mode := os.FileMode(header.Mode) & os.ModePerm // strip setuid/setgid/sticky
		if err := writeFileFromReaderDirect(tr, destPath, mode, selfName); err != nil {
			if limitedGz.N == 0 {
				return fmt.Errorf("archive exceeds maximum extracted size limit of %d bytes", maxTotalSize)
			}
			return err
		}
	}
	if limitedGz.N == 0 {
		return fmt.Errorf("archive exceeds maximum extracted size limit of %d bytes", maxTotalSize)
	}
	return nil
}

// safeArchiveFileName validates and sanitises an archive entry name, returning the
// base filename and true when the entry is safe to extract. It rejects absolute
// paths, path-traversal sequences (".."), null bytes, and drive-letter prefixes.
func safeArchiveFileName(archiveName string) (string, bool) {
	normalized := strings.ReplaceAll(archiveName, "\\", "/")
	if normalized == "" || strings.HasPrefix(normalized, "/") || strings.ContainsRune(normalized, 0) || strings.Contains(normalized, ":/") {
		return "", false
	}
	for _, segment := range strings.Split(normalized, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", false
		}
	}

	name := path.Base(normalized)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return "", false
	}
	return name, true
}

// archiveDestPath resolves the absolute destination path for an archive entry,
// verifying that the result stays within destDir to prevent path-traversal attacks.
func archiveDestPath(destDir, name string) (string, error) {
	cleanDestDir, err := filepath.Abs(destDir)
	if err != nil {
		return "", fmt.Errorf("resolve destination directory: %w", err)
	}

	safeName, ok := safeArchiveFileName(name)
	if !ok || safeName != name {
		return "", fmt.Errorf("%w: %q", domain.ErrUnsafeArchivePath, name)
	}

	destPath := filepath.Join(cleanDestDir, safeName)
	rel, err := filepath.Rel(cleanDestDir, destPath)
	if err != nil {
		return "", fmt.Errorf("validate destination path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive entry %s escapes destination directory", name)
	}

	return destPath, nil
}

// shouldRenameLockedFile reports whether the file should be renamed to ".old"
// before overwriting, rather than deleted in-place. This is necessary for the
// running executable and any platform shared libraries (.dll on Windows, .dylib
// on macOS) that cannot be deleted or truncated while in use.
func shouldRenameLockedFile(name, selfName string) bool {
	lowerName := strings.ToLower(name)
	if strings.EqualFold(name, selfName) {
		return true
	}
	if runtime.GOOS == "windows" && strings.HasSuffix(lowerName, ".dll") {
		return true
	}
	if runtime.GOOS == "darwin" && strings.HasSuffix(lowerName, ".dylib") {
		return true
	}
	return false
}

// writeFileFromReader opens the source via openFn and writes it to destPath.
func writeFileFromReader(openFn func() (io.ReadCloser, error), destPath string, mode os.FileMode, selfName string) error {
	src, err := openFn()
	if err != nil {
		return fmt.Errorf("failed to open archive entry: %w", err)
	}
	defer src.Close()

	return writeFileFromReaderDirect(src, destPath, mode, selfName)
}

// writeFileFromReaderDirect writes content from reader to destPath with the given permissions.
func writeFileFromReaderDirect(src io.Reader, destPath string, mode os.FileMode, selfName string) error {
	return writeFileFromReaderDirectWithLimit(src, destPath, mode, maxArchiveEntrySize, selfName)
}

// writeFileFromReaderDirectWithLimit is the testable core of writeFileFromReaderDirect,
// accepting an explicit maxFileSize cap so unit tests can exercise the per-entry
// size-limit path without constructing large payloads. It writes src to a sibling
// temporary file and atomically renames it into place via replaceFile.
func writeFileFromReaderDirectWithLimit(src io.Reader, destPath string, mode os.FileMode, maxFileSize int64, selfName string) error {
	if mode == 0 {
		mode = 0755
	}

	dir := filepath.Dir(destPath)
	base := filepath.Base(destPath)
	tmp, err := os.CreateTemp(dir, "."+base+".*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary file for %s: %w", destPath, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	limitedSrc := &io.LimitedReader{R: src, N: maxFileSize + 1}
	written, err := io.Copy(tmp, limitedSrc)
	if err != nil {
		tmp.Close()
		return fmt.Errorf("failed to write file %s: %w", destPath, err)
	}
	if written > maxFileSize || limitedSrc.N == 0 {
		tmp.Close()
		return fmt.Errorf("file %s exceeds maximum size limit of %d bytes", destPath, maxFileSize)
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to set file mode for %s: %w", destPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file for %s: %w", destPath, err)
	}

	if err := replaceFile(tmpPath, destPath, selfName); err != nil {
		return err
	}

	return nil
}

// replaceFile atomically moves tmpPath to destPath. For files that cannot be
// deleted while in use (see shouldRenameLockedFile), the existing file is first
// renamed to destPath+".old" so the rename of the new binary succeeds. On failure
// after a rename, the old file is restored.
func replaceFile(tmpPath, destPath, selfName string) error {
	name := filepath.Base(destPath)
	renamedOld := false
	oldPath := destPath + ".old"

	if shouldRenameLockedFile(name, selfName) {
		os.Remove(oldPath)
		if err := os.Rename(destPath, oldPath); err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("failed to rename locked file %s: %w", name, err)
			}
		} else {
			renamedOld = true
		}
	} else if err := os.Remove(destPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to replace file %s: %w", destPath, err)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		if renamedOld {
			_ = os.Rename(oldPath, destPath)
		}
		return fmt.Errorf("failed to move temporary file into place for %s: %w", destPath, err)
	}

	return nil
}

// writeReleaseNotes creates RELEASE_NOTES.md in the given directory with the release body.
func writeReleaseNotes(dir string, release *githubRelease) error {
	content := fmt.Sprintf("# Release %s\n\n**Published:** %s\n\n---\n\n%s\n",
		release.TagName,
		release.PublishedAt,
		release.Body,
	)

	path := filepath.Join(dir, "RELEASE_NOTES.md")
	return os.WriteFile(path, []byte(content), 0644)
}

// restartGracePeriod is how long we wait after StartProcess returns to confirm
// the child actually launched. If the child dies within this window — for
// example because the new binary is corrupt or a required shared library
// could not be loaded — we roll back to the .old backup instead of stranding
// the user with a broken install.
const restartGracePeriod = 500 * time.Millisecond

// restartSelf launches a new instance of the binary and exits the current process.
// The child is started in its own process group so signals sent to the parent's
// group (e.g. SIGHUP from a closing terminal) do not propagate to the new instance.
// If the child exits within restartGracePeriod the previous binary is restored
// from the .old backup and an error is returned without exiting the parent.
func restartSelf(selfPath string) error {
	args := os.Args
	env := os.Environ()
	oldPath := selfPath + ".old"

	// Record whether a .old backup exists so we know whether restoration is
	// possible if the new child fails to start.
	hadOldBackup := false
	if _, err := os.Stat(oldPath); err == nil {
		hadOldBackup = true
	}

	proc, err := os.StartProcess(selfPath, args, &os.ProcAttr{
		Dir:   filepath.Dir(selfPath),
		Env:   env,
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
		Sys:   detachSysProcAttr(),
	})
	if err != nil {
		return fmt.Errorf("failed to restart: %w", err)
	}

	// Wait briefly to confirm the child actually started.
	type waitResult struct {
		state *os.ProcessState
		err   error
	}
	waitCh := make(chan waitResult, 1)
	go func() {
		state, err := proc.Wait()
		waitCh <- waitResult{state: state, err: err}
	}()

	select {
	case result := <-waitCh:
		// A clean exit (e.g. the new version printed a one-shot message and
		// exited 0 within the grace period) is a successful run, not a failure
		// to start. The new binary is on disk; terminate the parent so the
		// running process matches the reported successful update.
		if result.err == nil && result.state.Success() {
			os.Exit(0)
		}
		if hadOldBackup {
			if restoreErr := restoreOldBackups(selfPath); restoreErr != nil {
				return fmt.Errorf("restart: child exited (state: %s) and restore failed: %w", result.state, restoreErr)
			}
		}
		if result.err != nil {
			return fmt.Errorf("restart: child process exited immediately: %w", result.err)
		}
		return fmt.Errorf("restart: child process exited immediately (state: %s)", result.state)
	case <-time.After(restartGracePeriod):
	}

	// Detach the child so it survives our exit.
	proc.Release()
	os.Exit(0)

	return nil // Unreachable, but satisfies the compiler
}

// restoreOldBackups renames the binary's ".old" backup and any sibling shared
// library ".old" backups in the same directory back to their original names.
// Mirrors the platform-specific pattern list used by CleanupOldBinary.
func restoreOldBackups(selfPath string) error {
	if err := os.Rename(selfPath+".old", selfPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("rename %s.old: %w", selfPath, err)
	}
	var patterns []string
	if runtime.GOOS == "windows" {
		patterns = []string{"*.dll.old"}
	} else {
		patterns = []string{"*.dylib.old"}
	}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(filepath.Dir(selfPath), pattern))
		if err != nil {
			continue
		}
		for _, match := range matches {
			dest := strings.TrimSuffix(match, ".old")
			// os.Rename on Windows fails if the destination already exists; the
			// replaceFile path overwrote it with the new file, so remove it first.
			if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove %s: %w", dest, err)
			}
			if err := os.Rename(match, dest); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("rename %s: %w", match, err)
			}
		}
	}
	return nil
}
