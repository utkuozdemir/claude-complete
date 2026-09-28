package spec_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/utkuozdemir/claude-complete/internal/spec"
)

func TestVersionFromPath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	for path, content := range map[string]string{
		"home/.local/share/claude/versions/2.1.283":                     "binary",
		"home/.local/share/claude/versions/latest":                      "binary",
		"opt/homebrew/Caskroom/claude-code/2.1.274/claude":              "binary",
		"opt/homebrew/Caskroom/claude-code@latest/2.1.283/claude":       "binary",
		"npm/lib/node_modules/@anthropic-ai/claude-code/bin/claude.exe": "binary",
		"npm/lib/node_modules/@anthropic-ai/claude-code/package.json":   `{"version": "2.1.280"}`,
		"home/bin/claude": "#!/bin/sh\nexec claude-real \"$@\"\n",
	} {
		path = filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// how each installer puts claude on PATH
	link := func(name, target string) string {
		path := filepath.Join(root, "links", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.Symlink(filepath.Join(root, target), path); err != nil {
			t.Fatal(err)
		}

		return path
	}

	for _, tc := range []struct {
		name, path, want string
	}{
		{"native installer", link("native", "home/.local/share/claude/versions/2.1.283"), "2.1.283"},
		{"homebrew cask", link("cask", "opt/homebrew/Caskroom/claude-code/2.1.274/claude"), "2.1.274"},
		{"homebrew latest cask", link("latest-cask", "opt/homebrew/Caskroom/claude-code@latest/2.1.283/claude"), "2.1.283"},
		{"not a version", link("latest", "home/.local/share/claude/versions/latest"), ""},
		// its manifest is not guaranteed to match the binary
		{"npm", link("npm", "npm/lib/node_modules/@anthropic-ai/claude-code/bin/claude.exe"), ""},
		{"wrapper script", filepath.Join(root, "home/bin/claude"), ""},
		{"missing", filepath.Join(root, "nowhere/claude"), ""},
	} {
		if got := spec.VersionFromPath(tc.path); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}
