package git

import "testing"

func TestComputeLineChangeCounts(t *testing.T) {
	tests := []struct {
		name     string
		old      string
		new      string
		add      int
		del      int
	}{
		{"identical", "a\nb\nc\n", "a\nb\nc\n", 0, 0},
		{"empty_to_content", "", "a\nb\n", 2, 0},
		{"content_to_empty", "a\nb\n", "", 0, 2},
		{"add_lines", "a\n", "a\nb\nc\n", 2, 0},
		{"remove_lines", "a\nb\nc\n", "a\n", 0, 2},
		{"modify_lines", "a\nb\nc\n", "a\nx\nc\n", 1, 1},
		{"both_empty", "", "", 0, 0},
		{"no_trailing_newline", "a\nb", "a\nc", 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			add, del := ComputeLineChangeCounts(tt.old, tt.new)
			if add != tt.add || del != tt.del {
				t.Errorf("ComputeLineChangeCounts() = (%d, %d), want (%d, %d)", add, del, tt.add, tt.del)
			}
		})
	}
}

func TestDiffPath(t *testing.T) {
	tests := []struct {
		name string
		diff Diff
		want string
	}{
		{"new_path", Diff{NewPath: "b.txt"}, "b.txt"},
		{"old_path_only", Diff{OldPath: "a.txt"}, "a.txt"},
		{"both_paths", Diff{OldPath: "a.txt", NewPath: "b.txt"}, "b.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DiffPath(&tt.diff)
			if got != tt.want {
				t.Errorf("DiffPath() = %q, want %q", got, tt.want)
			}
		})
	}
}
