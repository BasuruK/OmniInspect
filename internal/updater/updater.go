package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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

// CheckAndUpdate checks the latest GitHub release and prompts the user to update.
// currentVersion is the version string embedded in the binary at build time (e.g. "v0.2.0").
// Returns nil if no update is needed or the user declines. Returns an error on failure.
func CheckAndUpdate(currentVersion string) error {
	// Skip update check for development builds
	if currentVersion == "dev" || currentVersion == "" {
		fmt.Println("[updater] Development build detected — skipping update check.")
		return nil
	}

	fmt.Println("[updater] Checking for updates...")

	// Fetch the latest release from GitHub
	release, err := fetchLatestRelease()
	if err != nil {
		fmt.Printf("[updater] Could not check for updates: %v\n", err)
		return nil // Non-fatal: don't block the application from starting
	}

	// Compare versions
	if !isNewer(release.TagName, currentVersion) {
		fmt.Printf("[updater] You are on the latest version (%s).\n", currentVersion)
		return nil
	}

	// Prompt the user
	fmt.Printf("[updater] Update %s available (current: %s).\n", release.TagName, currentVersion)
	fmt.Print("[updater] Update now? [Y/n]: ")

	var answer string
	fmt.Scanln(&answer)
	answer = strings.TrimSpace(strings.ToLower(answer))

	if answer != "" && answer != "y" && answer != "yes" {
		fmt.Println("[updater] Update skipped.")
		return nil
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
		return fmt.Errorf("no matching release asset found for %s (expected %s)", runtime.GOOS+"/"+runtime.GOARCH, assetName)
	}

	fmt.Printf("[updater] Downloading %s...\n", assetName)

	// Download to a temporary file
	tmpFile, err := downloadToTemp(downloadURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer os.Remove(tmpFile) // Clean up the temp archive

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

	// Extract the archive into the same directory as the running binary
	fmt.Println("[updater] Extracting update...")
	if err := extractArchive(tmpFile, selfDir, selfPath); err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}

	// Write RELEASE_NOTES.md in the same directory as the binary
	if err := writeReleaseNotes(selfDir, release); err != nil {
		fmt.Printf("[updater] Warning: could not write RELEASE_NOTES.md: %v\n", err)
		// Non-fatal
	}

	fmt.Println("[updater] Update complete. Restarting...")

	// Restart the application
	return restartSelf(selfPath)
}

// CleanupOldBinary removes the ".old" leftover from a previous update, if it exists.
// Call this at the very top of main().
func CleanupOldBinary() {
	selfPath, err := os.Executable()
	if err != nil {
		return
	}
	selfPath, _ = filepath.EvalSymlinks(selfPath)
	oldPath := selfPath + ".old"
	if _, err := os.Stat(oldPath); err == nil {
		os.Remove(oldPath)
	}
}

// --------------------------------------------------------------------------
// Internal helpers
// --------------------------------------------------------------------------

// fetchLatestRelease calls the GitHub API and returns the latest stable release.
func fetchLatestRelease() (*githubRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", gitHubAPIBase, GitHubRepo)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
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
	osName := runtime.GOOS   // "windows" or "darwin"
	arch := runtime.GOARCH   // "amd64" or "arm64"

	if osName == "windows" {
		return fmt.Sprintf("omniview-%s-%s-%s.zip", osName, arch, tag)
	}
	return fmt.Sprintf("omniview-%s-%s-%s.tar.gz", osName, arch, tag)
}

// downloadToTemp downloads the given URL to a temporary file and returns its path.
func downloadToTemp(url string) (string, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
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

// extractArchive extracts a .zip or .tar.gz archive into destDir.
// selfPath is the path of the currently running binary — it gets renamed to .old
// before being overwritten.
func extractArchive(archivePath, destDir, selfPath string) error {
	if runtime.GOOS == "windows" {
		return extractZip(archivePath, destDir, selfPath)
	}
	return extractTarGz(archivePath, destDir, selfPath)
}

// extractZip extracts a .zip archive. If a file matches the running binary name,
// the current binary is renamed to .old first.
func extractZip(archivePath, destDir, selfPath string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}
	defer r.Close()

	selfName := filepath.Base(selfPath)

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}

		// Use only the base name — archives may contain a top-level directory
		name := filepath.Base(f.Name)
		destPath := filepath.Join(destDir, name)

		// If we're about to overwrite ourselves, rename the running binary first
		if strings.EqualFold(name, selfName) {
			if err := os.Rename(selfPath, selfPath+".old"); err != nil {
				return fmt.Errorf("failed to rename current binary: %w", err)
			}
		}

		if err := writeFileFromReader(f.Open, destPath, f.Mode()); err != nil {
			return err
		}
	}
	return nil
}

// extractTarGz extracts a .tar.gz archive. If a file matches the running binary name,
// the current binary is renamed to .old first.
func extractTarGz(archivePath, destDir, selfPath string) error {
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

	tr := tar.NewReader(gz)
	selfName := filepath.Base(selfPath)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read error: %w", err)
		}

		if header.Typeflag != tar.TypeReg {
			continue // Skip directories and special files
		}

		name := filepath.Base(header.Name)
		destPath := filepath.Join(destDir, name)

		// If we're about to overwrite ourselves, rename the running binary first
		if name == selfName {
			if err := os.Rename(selfPath, selfPath+".old"); err != nil {
				return fmt.Errorf("failed to rename current binary: %w", err)
			}
		}

		mode := os.FileMode(header.Mode)
		if err := writeFileFromReaderDirect(tr, destPath, mode); err != nil {
			return err
		}
	}
	return nil
}

// writeFileFromReader opens the source via openFn and writes it to destPath.
func writeFileFromReader(openFn func() (io.ReadCloser, error), destPath string, mode os.FileMode) error {
	src, err := openFn()
	if err != nil {
		return fmt.Errorf("failed to open archive entry: %w", err)
	}
	defer src.Close()

	return writeFileFromReaderDirect(src, destPath, mode)
}

// writeFileFromReaderDirect writes content from reader to destPath with the given permissions.
func writeFileFromReaderDirect(src io.Reader, destPath string, mode os.FileMode) error {
	if mode == 0 {
		mode = 0755
	}

	dst, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", destPath, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to write file %s: %w", destPath, err)
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

// restartSelf launches a new instance of the binary and exits the current process.
func restartSelf(selfPath string) error {
	args := os.Args
	env := os.Environ()

	proc, err := os.StartProcess(selfPath, args, &os.ProcAttr{
		Dir:   filepath.Dir(selfPath),
		Env:   env,
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	})
	if err != nil {
		return fmt.Errorf("failed to restart: %w", err)
	}

	// Detach the child so it survives our exit
	proc.Release()
	os.Exit(0)

	return nil // Unreachable, but satisfies the compiler
}
