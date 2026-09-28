package spec_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/utkuozdemir/claude-complete/internal/spec"
)

// FuzzParse feeds arbitrary text as the root help. No choice may be empty, and the cache must accept what
// the parser produced, otherwise every completion walks the help.
func FuzzParse(f *testing.F) {
	addSeeds(f)

	// a directory per input makes the fuzzing stall
	path := filepath.Join(f.TempDir(), "spec.json")

	f.Fuzz(func(t *testing.T, help string) {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}

		var walks int

		run := func(_ context.Context, args ...string) ([]byte, error) {
			switch {
			case slices.Equal(args, []string{"--version"}):
				return []byte("2.1.282"), nil
			case slices.Equal(args, []string{"--help"}):
				walks++

				return []byte(help), nil
			}

			return nil, errors.New("no such command")
		}

		fresh, err := spec.Load(t.Context(), run, path, "", false)
		if err != nil {
			return // not a claude help, which is a fine answer
		}

		checkChoices(t, fresh.Root)

		cached, err := spec.Load(t.Context(), run, path, "", false)
		if err != nil {
			t.Fatalf("the second load failed: %v", err)
		}

		if walks != 1 {
			t.Fatalf("the cache did not accept what the parser produced, the help was walked %d times", walks)
		}

		want, _ := json.Marshal(fresh)
		got, _ := json.Marshal(cached)

		if string(got) != string(want) {
			t.Fatalf("the cache changed the spec:\n%s\nwant\n%s", got, want)
		}
	})
}

// addSeeds adds every captured help to the seed corpus.
func addSeeds(f *testing.F) {
	f.Helper()

	seeds, err := filepath.Glob(filepath.Join("testdata", "*.txt"))
	if err != nil {
		f.Fatal(err)
	}

	for _, seed := range seeds {
		data, err := os.ReadFile(seed)
		if err != nil {
			f.Fatal(err)
		}

		f.Add(string(data))
	}
}

func checkChoices(t *testing.T, cmd *spec.Command) {
	t.Helper()

	for _, flag := range cmd.Flags {
		for _, choice := range flag.Choices {
			if choice == "" {
				t.Fatalf("%s has an empty choice", flag.Long())
			}
		}
	}

	for _, sub := range cmd.Commands {
		checkChoices(t, sub)
	}
}
