//go:build live

package spec_test

import (
	"testing"
	"time"

	"github.com/utkuozdemir/claude-complete/internal/spec"
)

// TestLive walks the claude installed on this machine, to catch a change in the help format.
func TestLive(t *testing.T) {
	t.Parallel()

	root, err := spec.Walk(t.Context(), spec.Exec("claude", 10*time.Second))
	if err != nil {
		t.Fatal(err)
	}

	var count func(cmd *spec.Command) int

	count = func(cmd *spec.Command) int {
		total := 1
		for _, sub := range cmd.Commands {
			total += count(sub)
		}

		return total
	}

	if nodes := count(root); nodes < 40 {
		t.Errorf("walked %d commands, want at least 40", nodes)
	}

	if mode := flag(t, root, "--permission-mode"); len(mode.Choices) != 6 {
		t.Errorf("--permission-mode has choices %q, want six", mode.Choices)
	}
}
