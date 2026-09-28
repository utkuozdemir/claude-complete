package tree_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/utkuozdemir/claude-complete/internal/spec"
	"github.com/utkuozdemir/claude-complete/internal/tree"
)

// realSpec returns the spec of claude 2.1.282.
func realSpec(tb testing.TB) *spec.Spec {
	tb.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "spec.json"))
	if err != nil {
		tb.Fatal(err)
	}

	var installed spec.Spec
	if err = json.Unmarshal(data, &installed); err != nil {
		tb.Fatal(err)
	}

	return &installed
}

// FuzzLine feeds arbitrary command lines, one word per space, to the scanner. Its output must not change
// when scanned again.
func FuzzLine(f *testing.F) {
	for _, seed := range []string{
		"",
		"--resume ",
		"-r mcp --mod",
		"--add-dir /tmp /usr --mod",
		"--add-dir=/tmp doc",
		"--add-dir /tmp --resume x /us",
		"--name mcp --prompt-suggestions t",
		"plugin marketplace add --sparse /tmp /us",
		"--tmux=classic --verbose mcp get ",
		"--model -- --resume x",
		"-pr x --fallback-model a,b ",
	} {
		f.Add(seed)
	}

	root := realSpec(f).Root

	f.Fuzz(func(t *testing.T, line string) {
		words := strings.Split(line, " ")

		once := tree.Normalize(root, words)
		if len(once) > len(words)-1 {
			t.Fatalf("scanning added words: %q became %q", words, once)
		}

		if twice := tree.Normalize(root, append(slices.Clone(once), words[len(words)-1])); !slices.Equal(twice, once) {
			t.Fatalf("scanning is not idempotent: %q became %q, then %q", words, once, twice)
		}
	})
}
