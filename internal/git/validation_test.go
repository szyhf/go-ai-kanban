package git

import "testing"

func TestIsValidBranchPrefix(t *testing.T) {
	tests := []struct {
		prefix string
		want   bool
	}{
		{"", true},
		{"feature", true},
		{"bugfix", true},
		{"hotfix", true},
		{"release", true},
		{"feat/123", false},  // contains slash
		{"feature branch", false}, // contains space
		{"test~", false},
		{"test^", false},
		{"test:", false},
		{"..test", false},
	}

	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			got := IsValidBranchPrefix(tt.prefix)
			if got != tt.want {
				t.Errorf("IsValidBranchPrefix(%q) = %v, want %v", tt.prefix, got, tt.want)
			}
		})
	}
}

func TestIsBranchNameValid(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"main", true},
		{"feature/test", true},
		{"feature-test", true},
		{"", false},
		{" ", false},
		{"test..name", false},
		{"test~name", false},
		{"test^name", false},
		{"test:name", false},
		{".test", false},
		{"/test", false},
		{"test/", false},
		{"test.lock", false},
		{"@", false},
		{"test@{0}", false},
		{"test.", false},
		{"a//b", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBranchNameValid(tt.name)
			if got != tt.want {
				t.Errorf("IsBranchNameValid(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
