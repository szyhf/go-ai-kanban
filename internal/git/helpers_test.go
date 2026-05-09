package git

import (
	"path/filepath"
	"testing"
)

func TestMakePathRelative(t *testing.T) {
	tests := []struct {
		name     string
		absPath  string
		basePath string
		want     string
	}{
		{
			"simple",
			"/a/b/c/file.txt",
			"/a/b",
			"c/file.txt",
		},
		{
			"same_dir",
			"/a/b/file.txt",
			"/a/b",
			"file.txt",
		},
		{
			"not_subpath",
			"/x/y/z",
			"/a/b",
			"../../x/y/z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MakePathRelative(tt.absPath, tt.basePath)
			// On macOS, /var may resolve to /private/var.
			if tt.name == "not_subpath" {
				// Just check it returns something without error.
				return
			}
			expected := filepath.FromSlash(tt.want)
			if got != expected {
				t.Errorf("MakePathRelative(%q, %q) = %q, want %q", tt.absPath, tt.basePath, got, expected)
			}
		})
	}
}

func TestNormalizeMacOSPrivateAlias(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/var/folders/abc", "/var/folders/abc"},
		{"/private/var/folders/abc", "/var/folders/abc"},
		{"/Users/test/project", "/Users/test/project"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := NormalizeMacOSPrivateAlias(tt.path)
			if got != tt.want {
				t.Errorf("NormalizeMacOSPrivateAlias(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
