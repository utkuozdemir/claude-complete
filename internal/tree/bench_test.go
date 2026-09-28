package tree_test

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/carapace-sh/carapace"

	"github.com/utkuozdemir/claude-complete/internal/tree"
)

// benchSessions is the number of transcripts, about as many as in a heavily used project.
const benchSessions = 110

// benchHome creates a `~/.claude` about the size of a real one and returns the project directory.
func benchHome(b *testing.B) string {
	b.Helper()

	root := b.TempDir()
	home := filepath.Join(root, "home")
	project := filepath.Join(root, "project")
	transcripts := filepath.Join(home, ".claude", "projects", strings.ReplaceAll(project, "/", "-"))

	// a message line of about 2 KB, the bulk of every transcript
	message := `{"type":"assistant","message":"` + strings.Repeat("x", 2000) + `"}` + "\n"
	body := strings.Repeat(message, 150)

	files := map[string]string{}

	for idx := range benchSessions {
		files[filepath.Join(transcripts, fmt.Sprintf("%08d-0000-4000-8000-000000000000.jsonl", idx))] = body +
			fmt.Sprintf(`{"type":"custom-title","customTitle":"session-%d"}`+"\n", idx) + message +
			fmt.Sprintf(`{"type":"pr-link","prNumber":%d,"prRepository":"org/repo"}`+"\n", idx) + message +
			fmt.Sprintf(`{"type":"ai-title","aiTitle":"Work on %d"}`+"\n", idx) + message
	}

	maps.Copy(files, benchConfig(b, home, project))

	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			b.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			b.Fatal(err)
		}
	}

	if err := os.MkdirAll(project, 0o755); err != nil {
		b.Fatal(err)
	}

	b.Setenv("HOME", home)
	b.Setenv("CLAUDE_CONFIG_DIR", "")

	return project
}

// benchConfig returns the CLI configuration files under home, about the size of real ones.
func benchConfig(b *testing.B, home, project string) map[string]string {
	b.Helper()

	projects := map[string]any{
		project: map[string]any{"mcpServers": map[string]any{"local": map[string]any{"command": "x"}}},
	}
	for idx := range 60 {
		projects[fmt.Sprintf("/work/project-%d", idx)] = map[string]any{"history": strings.Repeat("y", 3000)}
	}

	config, err := json.Marshal(map[string]any{
		"projects":     projects,
		"mcpServers":   map[string]any{"remote": map[string]any{"type": "http"}},
		"oauthAccount": map[string]any{"organizationUuid": "org"},
	})
	if err != nil {
		b.Fatal(err)
	}

	plugins := make([]map[string]string, 0, 315)
	for idx := range 315 {
		plugins = append(plugins, map[string]string{"name": fmt.Sprintf("plugin-%d", idx), "description": "A plugin"})
	}

	catalog, err := json.Marshal(map[string]any{"plugins": plugins})
	if err != nil {
		b.Fatal(err)
	}

	marketplace := filepath.Join(home, ".claude", "plugins", "marketplaces", "official", ".claude-plugin")

	return map[string]string{
		filepath.Join(home, ".claude.json"):                                  string(config),
		filepath.Join(home, ".claude", "plugins", "known_marketplaces.json"): `{"official": {}}`,
		filepath.Join(marketplace, "marketplace.json"):                       string(catalog),
	}
}

// BenchmarkSources measures each data source. Closing the freshly written files costs more than closing
// real ones, so compare the numbers, do not read them as a completion time.
func BenchmarkSources(b *testing.B) {
	project := benchHome(b)
	sources := tree.Sources("2.1.282")

	for _, bench := range []struct{ name, source, prefix string }{
		{"sessions", "sessions", ""},
		{"sessions by prefix", "sessions", "session-1"},
		{"pull requests", "pull requests", ""},
		{"mcp servers", "mcp servers", ""},
		{"plugins to install", "plugins to install", ""},
		{"projects", "projects", ""},
		{"model options", "model options", ""},
		{"model catalog", "model catalog", ""},
	} {
		b.Run(bench.name, func(b *testing.B) {
			ctx := carapace.Context{Dir: project, Value: bench.prefix}

			b.ReportAllocs()

			for b.Loop() {
				sources[bench.source](ctx)
			}
		})
	}
}
