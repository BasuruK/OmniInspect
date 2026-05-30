package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeArchiveFileName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		archiveName string
		want        string
		wantOK      bool
	}{
		{name: "plain file", archiveName: "omniview", want: "omniview", wantOK: true},
		{name: "top-level directory", archiveName: "OmniView/omniview", want: "omniview", wantOK: true},
		{name: "absolute path", archiveName: "/tmp/omniview", wantOK: false},
		{name: "windows absolute path", archiveName: "C:\\tmp\\omniview.exe", wantOK: false},
		{name: "parent traversal", archiveName: "../omniview", wantOK: false},
		{name: "nested traversal", archiveName: "release/../omniview", wantOK: false},
		{name: "windows traversal", archiveName: "..\\omniview.exe", wantOK: false},
		{name: "empty", archiveName: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := safeArchiveFileName(tt.archiveName)
			if ok != tt.wantOK {
				t.Fatalf("safeArchiveFileName(%q) ok = %v, want %v", tt.archiveName, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("safeArchiveFileName(%q) = %q, want %q", tt.archiveName, got, tt.want)
			}
		})
	}
}

func TestWriteFileFromReaderDirectOversizedDoesNotReplaceExistingFile(t *testing.T) {
	dir := t.TempDir()
	destPath := filepath.Join(dir, "omniview")
	if err := os.WriteFile(destPath, []byte("original"), 0644); err != nil {
		t.Fatalf("write original file: %v", err)
	}

	err := writeFileFromReaderDirectWithLimit(strings.NewReader("12345"), destPath, 0644, 4, "omniview")
	if err == nil {
		t.Fatal("writeFileFromReaderDirect() error = nil, want size limit error")
	}

	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read destination after failed write: %v", err)
	}
	if string(content) != "original" {
		t.Fatalf("destination content = %q, want original file preserved", content)
	}
	if _, err := os.Stat(destPath + ".old"); !os.IsNotExist(err) {
		t.Fatalf("oversized locked file created .old backup, stat error = %v", err)
	}
}

func TestArchiveDestPathRejectsUnsafeArchiveNames(t *testing.T) {
	t.Parallel()

	destDir := t.TempDir()
	tests := []string{
		"../omniview",
		"release/omniview",
		"/tmp/omniview",
		"C:\\tmp\\omniview.exe",
	}

	for _, name := range tests {
		name := name
		t.Run(name, func(t *testing.T) {
			if _, err := archiveDestPath(destDir, name); err == nil {
				t.Fatalf("archiveDestPath(%q) error = nil, want unsafe path error", name)
			}
		})
	}
}

func TestArchiveDestPathAcceptsSafeFileName(t *testing.T) {
	t.Parallel()

	destDir := t.TempDir()
	destPath, err := archiveDestPath(destDir, "omniview")
	if err != nil {
		t.Fatalf("archiveDestPath() error = %v", err)
	}
	if got, want := destPath, filepath.Join(destDir, "omniview"); got != want {
		t.Fatalf("archiveDestPath() = %q, want %q", got, want)
	}
}

func TestExtractTarGzRejectsArchiveExceedingTotalSizeLimit(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "update.tar.gz")
	writeTestTarGzArchive(t, archivePath,
		testArchiveFile{name: "omniview", content: "1234"},
		testArchiveFile{name: "libupdate.dylib", content: "5678"},
	)

	destDir := t.TempDir()
	err := extractTarGzWithLimit(archivePath, destDir, filepath.Join(destDir, "omniview"), 7)
	if err == nil {
		t.Fatal("extractTarGzWithLimit() error = nil, want total size limit error")
	}
	if !strings.Contains(err.Error(), "maximum extracted size limit") {
		t.Fatalf("extractTarGzWithLimit() error = %v, want total size limit error", err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "libupdate.dylib")); !os.IsNotExist(err) {
		t.Fatalf("second file should not be extracted after size limit, stat error = %v", err)
	}
}

func TestExtractZipRejectsArchiveExceedingTotalSizeLimit(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "update.zip")
	writeTestZipArchive(t, archivePath,
		testArchiveFile{name: "omniview.exe", content: "1234"},
		testArchiveFile{name: "helper.dll", content: "5678"},
	)

	destDir := t.TempDir()
	err := extractZipWithLimit(archivePath, destDir, filepath.Join(destDir, "omniview.exe"), 7)
	if err == nil {
		t.Fatal("extractZipWithLimit() error = nil, want total size limit error")
	}
	if !strings.Contains(err.Error(), "maximum extracted size limit") {
		t.Fatalf("extractZipWithLimit() error = %v, want total size limit error", err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "helper.dll")); !os.IsNotExist(err) {
		t.Fatalf("second file should not be extracted after size limit, stat error = %v", err)
	}
}

type testArchiveFile struct {
	name    string
	content string
}

func writeTestTarGzArchive(t *testing.T, archivePath string, files ...testArchiveFile) {
	t.Helper()

	archiveFile, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create tar.gz archive: %v", err)
	}

	gz := gzip.NewWriter(archiveFile)
	tw := tar.NewWriter(gz)

	for _, file := range files {
		header := &tar.Header{Name: file.name, Mode: 0644, Size: int64(len(file.content))}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := io.WriteString(tw, file.content); err != nil {
			t.Fatalf("write tar content: %v", err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	if err := archiveFile.Close(); err != nil {
		t.Fatalf("close tar.gz archive: %v", err)
	}
}

func writeTestZipArchive(t *testing.T, archivePath string, files ...testArchiveFile) {
	t.Helper()

	archiveFile, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create zip archive: %v", err)
	}

	zw := zip.NewWriter(archiveFile)
	for _, file := range files {
		entry, err := zw.Create(file.name)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := io.WriteString(entry, file.content); err != nil {
			t.Fatalf("write zip content: %v", err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := archiveFile.Close(); err != nil {
		t.Fatalf("close zip archive: %v", err)
	}
}
