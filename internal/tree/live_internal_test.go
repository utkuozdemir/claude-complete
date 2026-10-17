//go:build live

package tree

import (
	"strings"
	"testing"
	"time"

	"github.com/utkuozdemir/claude-complete/internal/spec"
)

// TestLive checks that the flags and commands completed from real data still exist in the installed claude.
func TestLive(t *testing.T) {
	t.Parallel()

	root, err := spec.Walk(t.Context(), spec.Exec("claude", 10*time.Second))
	if err != nil {
		t.Fatal(err)
	}

	names := map[string]bool{}

	var collect func(cmd *spec.Command)

	collect = func(cmd *spec.Command) {
		for _, flag := range cmd.Flags {
			names[flag.Long()] = true
		}

		for _, sub := range cmd.Commands {
			collect(sub)
		}
	}

	collect(root)

	for flag := range flagSources {
		if !names[flag] {
			t.Errorf("flag %s is not in the help anymore", flag)
		}
	}

	for path := range commandSources {
		if !hasPath(root, strings.Fields(path)) {
			t.Errorf("command %q is not in the help anymore", path)
		}
	}

	if models := newCompleter(&spec.Spec{Root: root}).models; len(models) == 0 {
		t.Error("the --model help quotes no model aliases anymore")
	}
}

// hasPath reports whether names is a path of primary command names under cmd, the way the tree builds it.
func hasPath(cmd *spec.Command, names []string) bool {
	if len(names) == 0 {
		return true
	}

	for _, sub := range cmd.Commands {
		if sub.Name == names[0] {
			return hasPath(sub, names[1:])
		}
	}

	return false
}
