package updater

import (
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
