package spec_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/utkuozdemir/claude-complete/internal/spec"
)

// BenchmarkLoadCached measures loading a current cached spec, which every completion does.
func BenchmarkLoadCached(b *testing.B) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "spec.json"))
	if err != nil {
		b.Fatal(err)
	}

	var cached spec.Spec
	if err = json.Unmarshal(data, &cached); err != nil {
		b.Fatal(err)
	}

	cached.Binary = "/usr/local/bin/claude 1 1"

	if data, err = json.Marshal(cached); err != nil {
		b.Fatal(err)
	}

	path := filepath.Join(b.TempDir(), "spec.json")
	if err = os.WriteFile(path, data, 0o600); err != nil {
		b.Fatal(err)
	}

	run := func(context.Context, ...string) ([]byte, error) {
		return nil, errors.New("a current cache must not run claude")
	}

	b.ReportAllocs()

	for b.Loop() {
		if _, err := spec.Load(b.Context(), run, path, cached.Binary, false); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkWalk measures parsing every captured help, without starting claude.
func BenchmarkWalk(b *testing.B) {
	run := fixtureRunner(b)

	b.ReportAllocs()

	for b.Loop() {
		if _, err := spec.Walk(b.Context(), run); err != nil {
			b.Fatal(err)
		}
	}
}
