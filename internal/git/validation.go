package git

import (
	"strings"
)

// IsValidBranchPrefix checks whether prefix is a valid git branch prefix.
// It validates by constructing "{prefix}/x" and checking git branch naming rules.
// An empty prefix is valid (means no prefix restriction).
func IsValidBranchPrefix(prefix string) bool {
	if prefix == "" {
		return true
	}

	// Slashes in prefix are not allowed because they create ambiguous hierarchy.
	if strings.Contains(prefix, "/") {
		return false
	}

	// Validate the full test branch name.
	testName := prefix + "/x"
	return isBranchNameValid(testName)
}

// isBranchNameValid checks whether a name is valid as a git branch name.
// Rules derived from git-check-ref-format.
func isBranchNameValid(name string) bool {
	if len(name) == 0 {
		return false
	}

	// Disallowed patterns.
	forbidden := []string{" ", "~", "^", ":", "?", "*", "[", "\\", ".."}
	for _, ch := range forbidden {
		if strings.Contains(name, ch) {
			return false
		}
	}

	// Cannot end with .lock
	if strings.HasSuffix(name, ".lock") {
		return false
	}

	// Cannot start with '.' or '/'
	if name[0] == '.' || name[0] == '/' {
		return false
	}

	// Cannot end with '/'
	if name[len(name)-1] == '/' {
		return false
	}

	// Cannot contain consecutive slashes
	if strings.Contains(name, "//") {
		return false
	}

	// Cannot end with '.'
	if name[len(name)-1] == '.' {
		return false
	}

	// Cannot contain '@{'
	if strings.Contains(name, "@{") {
		return false
	}

	// Cannot be a single '@'
	if name == "@" {
		return false
	}

	return true
}

// IsBranchNameValid is the exported version for external use.
func IsBranchNameValid(name string) bool {
	return isBranchNameValid(name)
}
