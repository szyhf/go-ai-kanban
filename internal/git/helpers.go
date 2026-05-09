package git

import (
	"path/filepath"
	"strings"
)

// AlwaysSkipDirs lists directories to exclude from git operations (e.g. diffs).
var AlwaysSkipDirs = []string{
	"node_modules",
	".next",
	".cache",
	"target",
	"dist",
	"build",
	".turbo",
	"coverage",
}

// MakePathRelative converts an absolute path to a relative path from basePath.
// Returns the original path if it cannot be made relative.
func MakePathRelative(absPath, basePath string) string {
	rel, err := filepath.Rel(basePath, absPath)
	if err != nil {
		return absPath
	}
	return rel
}

// NormalizeMacOSPrivateAlias resolves macOS /private/var -> /var and similar aliases.
func NormalizeMacOSPrivateAlias(path string) string {
	if strings.HasPrefix(path, "/private/") {
		return path[8:] // strip "/private" prefix
	}
	return path
}
