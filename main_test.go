package main_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// bin is the claude-complete binary under test.
var bin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "claude-complete-test-")
	if err != nil {
		panic(err)
	}

	bin = filepath.Join(dir, "claude-complete")

	if out, err := exec.CommandContext(context.Background(), "go", "build", "-o", bin, ".").
		CombinedOutput(); err != nil {
		panic(fmt.Sprintf("build failed: %v\n%s", err, out))
	}

	code := m.Run()

	os.RemoveAll(dir)

	os.Exit(code)
}

// machine has claude 2.1.282 installed and a fabricated ~/.claude.
type machine struct {
	home, cache, dir, path string
}

// fakeClaude answers only --version, so that a test that walks fails.
const fakeClaude = `#!/bin/sh
echo "$*" >> "$HOME/claude-calls"
if [ "$1" = "--version" ]; then echo "2.1.282 (Claude Code)"; exit 0; fi
echo "unexpected claude $*" >&2; exit 1
`

func newMachine(t *testing.T) machine {
	t.Helper()

	root := t.TempDir()

	mach := machine{
		home:  filepath.Join(root, "home"),
		cache: filepath.Join(root, "cache"),
		// underscores and dots become dashes too in the transcript directory name
		dir:  filepath.Join(root, "my_project.v2"),
		path: filepath.Join(root, "bin"),
	}

	// entered through a symlink, while the CLI records the resolved path
	if err := os.MkdirAll(filepath.Join(root, "real", "my_project.v2"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(filepath.Join(root, "real", "my_project.v2"), mach.dir); err != nil {
		t.Fatal(err)
	}

	spec, err := os.ReadFile(filepath.Join("testdata", "spec.json"))
	if err != nil {
		t.Fatal(err)
	}

	projectPath, err := filepath.EvalSymlinks(mach.dir)
	if err != nil {
		t.Fatal(err)
	}

	files := claudeData(mach.home, projectPath)
	files[filepath.Join(mach.path, "claude")] = fakeClaude
	files[filepath.Join(mach.cache, "claude-complete", "spec.json")] = string(spec)
	files[filepath.Join(mach.dir, ".mcp.json")] = `{"mcpServers": {"shared": {"type": "http"}}}`

	project := filepath.Join(mach.home, ".claude", "projects", mangle(projectPath))
	for name, content := range transcripts() {
		files[filepath.Join(project, name)] = content
	}

	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	return mach
}

// claudeData returns the CLI data under home, where every source has an entry in an unexpected shape.
func claudeData(home, projectPath string) map[string]string {
	return map[string]string{
		filepath.Join(home, ".claude.json"): `{
			"mcpServers": {"pylon": {"type": "http"}, "odd": 3},
			"projects": {` + quote(projectPath) + `: {"mcpServers": {"local-db": {"command": "db-mcp"}}},
				"/other/project": {}},
			"additionalModelOptionsCache": [{"value": "claude-fable-5-1[1m]", "description": "Fable 5.1"}, 42, {}],
			"oauthAccount": {"organizationUuid": "org-a", "accountUuid": "acct-1"}}`,
		// the catalog of the account
		catalogPath(home, "org-a", "acct-1", "cc"): `{"catalog": {"config": {"models": [
			{"id": "claude-opus-5-5", "name": "Opus 5.5", "description": "Most capable",
				"min_claude_code_version": "2.1.280"},
			{"id": "claude-sonnet-5", "name": "Sonnet 5"},
			{"id": "claude-masked", "name": "Masked"},
			{"id": "claude-future", "name": "Future", "min_claude_code_version": "9.0.0"},
			{"id": "claude-unreadable-min", "name": "Unreadable", "min_claude_code_version": "soon"},
			{"id": "Bad ID!", "name": "Bad"},
			7]}}}`,
		// the catalogs of another account, of another surface and of another organization
		catalogPath(home, "org-a", "acct-2", "cc"):  `{"catalog": {"config": {"models": [{"id": "claude-other-account"}]}}}`,
		catalogPath(home, "org-a", "acct-1", "ccd"): `{"catalog": {"config": {"models": [{"id": "claude-desktop"}]}}}`,
		catalogPath(home, "org-b", "acct-1", "cc"):  `{"catalog": {"config": {"models": [{"id": "claude-other-org"}]}}}`,
		filepath.Join(home, ".claude", "cache", "model-catalog", "masked-ids.json"): `{
			"version": 1, "ids": ["claude-masked"]}`,
		filepath.Join(home, ".claude", "plugins", "installed_plugins.json"): `{"version": 2, "plugins": {
			"gopls-lsp@official": [{"version": "1.0.0", "scope": "user"}], "weird@official": "not-a-list"}}`,
		filepath.Join(home, ".claude", "plugins", "known_marketplaces.json"): `{
			"official": {"source": {"repo": "anthropics/plugins"}}, "broken": 7}`,
		filepath.Join(home, ".claude", "plugins", "marketplaces", "official", ".claude-plugin",
			"marketplace.json"): `{"plugins": [{"name": "swift-lsp", "description": "Swift"}, 5, {"description": "x"}]}`,
		filepath.Join(home, ".claude", "agents", "reviewer.md"):          "---\nname: code-reviewer\n---\nbody\n",
		filepath.Join(home, ".claude", "agents", "plain.md"):             "no front matter\n",
		filepath.Join(home, ".claude", "jobs", "a5b6afd4", "state.json"): `{"name": "task", "state": "done"}`,
	}
}

// catalogPath returns the path of the model catalog of an organization, account and surface.
func catalogPath(home, organization, account, surface string) string {
	scope := sha256.Sum256([]byte(account))

	return filepath.Join(home, ".claude", "cache", "model-catalog",
		organization+"-"+hex.EncodeToString(scope[:])[:12]+"-"+surface+".json")
}

// transcripts returns the sessions of the test project, by file name.
func transcripts() map[string]string {
	transcript := func(records ...string) string {
		return `{"type":"user","message":"hello"}` + "\n" + strings.Join(records, "\n") + "\n"
	}

	return map[string]string{
		// the last title wins
		"11111111-1111-4111-8111-111111111111.jsonl": transcript(
			`{"type":"custom-title","customTitle":"old-name"}`,
			`{"type":"ai-title","aiTitle":"Talos client cache"}`,
			`{"type":"custom-title","customTitle":"talos-client-cache"}`,
			`{"type":"pr-link","prNumber":3368,"prRepository":"siderolabs/omni"}`),
		"22222222-2222-4222-8222-222222222222.jsonl": transcript(`{"type":"ai-title","aiTitle":"Untitled work"}`),
		"33333333-3333-4333-8333-333333333333.jsonl": transcript(`{"type":"custom-title","customTitle":"mcp"}`),
		// a fork shares its title with the original
		"44444444-4444-4444-8444-444444444444.jsonl": transcript(
			`{"type":"custom-title","customTitle":"talos-client-cache"}`),
		// linked before the part of the transcript that is read
		"55555555-5555-4555-8555-555555555555.jsonl": transcript(
			`{"type":"pr-link","prNumber":1,"prRepository":"siderolabs/omni"}`,
			strings.Repeat(`{"type":"user","message":"padding"}`+"\n", 3000)),
		"11111111-1111-4111-8111-111111111111.orphaned-1-abc.jsonl": transcript(
			`{"type":"custom-title","customTitle":"orphan"}`),
	}
}

func quote(s string) string {
	out, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}

	return string(out)
}

func mangle(path string) string {
	return strings.Map(func(r rune) rune {
		if r < 128 && (r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return r
		}

		return '-'
	}, path)
}

// env returns what a shell passes on, with `PWD` as entered, through the symlink.
func (mach machine) env(extra ...string) []string {
	return append([]string{
		"HOME=" + mach.home,
		"XDG_CACHE_HOME=" + mach.cache,
		"PATH=" + mach.path + ":/usr/bin:/bin",
		"PWD=" + mach.dir,
	}, extra...)
}

// complete returns the completions of the command line, whose last word is under the cursor.
func (mach machine) complete(t *testing.T, words ...string) []string {
	t.Helper()

	return mach.completeWith(t, nil, words...)
}

// completeWith is complete with extra environment variables.
func (mach machine) completeWith(t *testing.T, env []string, words ...string) []string {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), bin, append([]string{"_carapace", "export", "claude"}, words...)...)
	cmd.Dir = mach.dir
	cmd.Env = mach.env(env...)

	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%q: %v", words, err)
	}

	var export struct {
		Values []struct {
			Value string `json:"value"`
		} `json:"values"`
	}

	if err := json.Unmarshal(out, &export); err != nil {
		t.Fatalf("%q: %v\n%s", words, err, out)
	}

	values := make([]string, 0, len(export.Values))
	for _, v := range export.Values {
		values = append(values, v.Value)
	}

	slices.Sort(values)

	return values
}

type completionCase struct {
	name  string
	words []string
	want  []string
}

var staticCases = []completionCase{
	{"subcommands", []string{"doc"}, []string{"doctor"}},
	{"flags", []string{"--perm"}, []string{"--permission-mode", "--permission-prompts"}},
	{"choices with a space", []string{"--permission-mode", "pl"}, []string{"plan"}},
	{"choices with an equals sign", []string{"--permission-mode=pl"}, []string{"--permission-mode=plan"}},
	{"prose choices", []string{"--effort", "x"}, []string{"xhigh"}},
	{"comma-separated choices", []string{"--setting-sources", "user,pro"}, []string{"user,project"}},
	{"models from the help, the cached options and the catalog", []string{"--model", ""}, []string{
		"claude-fable-5", "claude-fable-5-1[1m]", "claude-opus-5-5", "claude-sonnet-5", "fable", "opus", "sonnet",
	}},
	{"comma-separated models", []string{"--fallback-model", "opus,so"}, []string{"opus,sonnet"}},
	{"subcommand flag values", []string{"agents", "--model", "op"}, []string{"opus"}},
	// the help of these flags lists no values
	{"borrowed choices", []string{"agents", "--effort", "x"}, []string{"xhigh"}},
	{"borrowed choices of a subcommand flag", []string{"plugin", "update", "--scope", "pro"}, []string{"project"}},
	{"after the separator", []string{"--", "--pe"}, []string{}},
	// a flag missing from the help
	{"unknown flag", []string{"--system-prompt-file", "p.md", "--mo"}, []string{"--model"}},
	// the help lists --tmux without a value and documents --tmux=classic
	{"value for a flag listed without one", []string{"--tmux=classic", "--mod"}, []string{"--model"}},
	{"flag without a value", []string{"--verbose", "doc"}, []string{"doctor"}},
}

var dataCases = []completionCase{
	// the ids of the sessions without a title, and no orphaned transcript
	{"sessions", []string{"--resume", ""}, []string{
		"22222222-2222-4222-8222-222222222222", "55555555-5555-4555-8555-555555555555", "mcp", "talos-client-cache",
	}},
	{"short flag", []string{"-r", "ta"}, []string{"talos-client-cache"}},
	{"equals sign", []string{"--resume=ta"}, []string{"--resume=talos-client-cache"}},
	{"pull requests within the tail", []string{"--from-pr", ""}, []string{"3368"}},
	{"agents", []string{"--agent", ""}, []string{"code-reviewer", "plain"}},
	{"mcp servers", []string{"mcp", "get", ""}, []string{"local-db", "odd", "pylon", "shared"}},
	{"remote mcp servers", []string{"mcp", "login", ""}, []string{"odd", "pylon", "shared"}},
	{"projects", []string{"project", "purge", "/oth"}, []string{"/other/project"}},
	{"installed plugins", []string{"plugin", "enable", ""}, []string{"gopls-lsp@official", "weird@official"}},
	{"plugins to install", []string{"plugin", "install", ""}, []string{"swift-lsp@official"}},
	{"marketplaces", []string{"plugin", "marketplace", "rm", ""}, []string{"broken", "official"}},
	{"jobs", []string{"stop", ""}, []string{"a5b6afd4"}},
	{"jobs through an alias", []string{"kill", ""}, []string{"a5b6afd4"}},
}

// commanderCases cover the optional and the variadic values, which pflag lacks.
var commanderCases = []completionCase{
	{"optional value then a flag", []string{"--resume", "--mod"}, []string{"--model"}},
	{"optional value then a flag value", []string{"--resume", "--model", "s"}, []string{"sonnet"}},
	{"optional value given", []string{"--resume", "talos-client-cache", "--mod"}, []string{"--model"}},
	{"value named like a subcommand", []string{"--resume", "mcp", "--mod"}, []string{"--model"}},
	{"short value named like a subcommand", []string{"-r", "mcp", "--mod"}, []string{"--model"}},
	{"variadic value", []string{"--add-dir", "/tmp", "/us"}, []string{"/usr/"}},
	{
		"variadic value of a subcommand",
		[]string{"plugin", "marketplace", "add", "--sparse", "/tmp", "/us"},
		[]string{"/usr/"},
	},
	// a variadic written with "=" takes one value only
	{"after an equals sign", []string{"--add-dir=/tmp", "doc"}, []string{"doctor"}},
	{"variadic value named like a subcommand", []string{"--add-dir", "/tmp", "mcp", "--mod"}, []string{"--model"}},
	{"after a variadic run", []string{"--add-dir", "/tmp", "/usr", "--mod"}, []string{"--model"}},
	{"optional value ends a variadic run", []string{"--add-dir", "/tmp", "--resume", "x", "/us"}, []string{}},
	{
		"required value named like a subcommand",
		[]string{"--name", "mcp", "--prompt-suggestions", "t"},
		[]string{"true"},
	},
}

func TestCompletion(t *testing.T) {
	t.Parallel()

	mach := newMachine(t)

	for _, tc := range slices.Concat(staticCases, dataCases, commanderCases) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := mach.complete(t, tc.words...); !slices.Equal(got, tc.want) {
				t.Errorf("claude %s<TAB>: got %q, want %q", strings.Join(tc.words, " "), got, tc.want)
			}
		})
	}
}

// TestConfigDir checks that `CLAUDE_CONFIG_DIR` moves every source.
func TestConfigDir(t *testing.T) {
	t.Parallel()

	mach := newMachine(t)
	profile := t.TempDir()

	files := map[string]string{
		filepath.Join(profile, "jobs", "b82c812f", "state.json"): `{"name": "other", "state": "running"}`,
		filepath.Join(profile, ".claude.json"):                   `{"mcpServers": {"work-only": {"type": "http"}}}`,
	}

	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	env := []string{"CLAUDE_CONFIG_DIR=" + profile}

	for _, tc := range []completionCase{
		{"jobs", []string{"stop", ""}, []string{"b82c812f"}},
		{"mcp servers", []string{"mcp", "get", ""}, []string{"shared", "work-only"}},
		{"installed plugins", []string{"plugin", "enable", ""}, []string{}},
		{"sessions", []string{"--resume", ""}, []string{}},
	} {
		if got := mach.completeWith(t, env, tc.words...); !slices.Equal(got, tc.want) {
			t.Errorf("%s: claude %s<TAB>: got %q, want %q", tc.name, strings.Join(tc.words, " "), got, tc.want)
		}
	}
}

// TestBrokenData checks that an unreadable source affects only itself.
func TestBrokenData(t *testing.T) {
	t.Parallel()

	mach := newMachine(t)

	if err := os.WriteFile(filepath.Join(mach.home, ".claude.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []completionCase{
		{"mcp servers", []string{"mcp", "get", ""}, []string{"shared"}},
		{"models", []string{"--model", ""}, []string{"claude-fable-5", "fable", "opus", "sonnet"}},
		{"projects", []string{"project", "purge", ""}, []string{}},
		{"jobs", []string{"stop", ""}, []string{"a5b6afd4"}},
	} {
		if got := mach.complete(t, tc.words...); !slices.Equal(got, tc.want) {
			t.Errorf("%s: claude %s<TAB>: got %q, want %q", tc.name, strings.Join(tc.words, " "), got, tc.want)
		}
	}
}

// TestDamagedCache checks that a damaged cached spec is not used. The fake claude cannot be walked, so no
// completions are expected.
func TestDamagedCache(t *testing.T) {
	t.Parallel()

	for name, content := range map[string]string{
		"not json":           "{not json",
		"flag without names": `{"version": "2.1.282", "root": {"flags": [{"names": []}]}}`,
		"null subcommand":    `{"version": "2.1.282", "root": {"commands": [null]}}`,
	} {
		mach := newMachine(t)

		err := os.WriteFile(filepath.Join(mach.cache, "claude-complete", "spec.json"), []byte(content), 0o600)
		if err != nil {
			t.Fatal(err)
		}

		if got := mach.complete(t, "--resume", ""); len(got) != 0 {
			t.Errorf("%s: got %q, want nothing", name, got)
		}
	}
}

// TestCompletionStartsNoProcess checks that claude runs only when its binary changed.
func TestCompletionStartsNoProcess(t *testing.T) {
	t.Parallel()

	mach := newMachine(t)
	calls := func() int {
		data, _ := os.ReadFile(filepath.Join(mach.home, "claude-calls"))

		return strings.Count(string(data), "\n")
	}

	mach.complete(t, "--perm")
	mach.complete(t, "--perm")

	if got := calls(); got != 1 {
		t.Fatalf("two completions ran claude %d times, want one version check", got)
	}

	// a reinstall changes the binary's modification time
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(mach.path, "claude"), later, later); err != nil {
		t.Fatal(err)
	}

	mach.complete(t, "--perm")

	if got := calls(); got != 2 {
		t.Fatalf("a changed binary ran claude %d times in total, want a second version check", got)
	}
}

func TestSnippetNeedsNoClaude(t *testing.T) {
	t.Parallel()

	for _, shell := range []string{"zsh", "bash", "fish"} {
		cmd := exec.CommandContext(t.Context(), bin, "_carapace", shell)
		// no claude on PATH and no cache, as at shell startup
		cmd.Env = []string{"HOME=" + t.TempDir(), "XDG_CACHE_HOME=" + t.TempDir(), "PATH="}

		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("%s: %v", shell, err)
		}

		if !strings.Contains(string(out), "claude-complete _carapace "+shell) {
			t.Errorf("%s: no completion function in the snippet:\n%s", shell, out)
		}
	}
}
