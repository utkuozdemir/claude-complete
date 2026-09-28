package spec_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/utkuozdemir/claude-complete/internal/spec"
)

// fixtureRunner returns the help outputs captured from claude 2.1.282, and the root help for any other path,
// as claude does.
func fixtureRunner(tb testing.TB, fail ...string) spec.Runner {
	tb.Helper()

	return func(_ context.Context, args ...string) ([]byte, error) {
		if slices.Equal(args, []string{"--version"}) {
			return []byte("2.1.282 (Claude Code)\n"), nil
		}

		path := strings.Join(args[:len(args)-1], " ")
		if slices.Contains(fail, path) {
			return nil, context.DeadlineExceeded
		}

		name := strings.ReplaceAll(path, " ", "_")
		if name == "" {
			name = "root"
		}

		data, err := os.ReadFile(filepath.Join("testdata", name+".txt"))
		if errors.Is(err, os.ErrNotExist) {
			data, err = os.ReadFile(filepath.Join("testdata", "root.txt"))
		}

		if err != nil {
			tb.Fatal(err)
		}

		return data, nil
	}
}

func walk(t *testing.T, fail ...string) *spec.Command {
	t.Helper()

	root, err := spec.Walk(t.Context(), fixtureRunner(t, fail...))
	if err != nil {
		t.Fatal(err)
	}

	return root
}

// paths lists every command of the tree with its aliases, depth first.
func paths(cmd *spec.Command, prefix string) []string {
	out := make([]string, 0, len(cmd.Commands))

	for _, sub := range cmd.Commands {
		p := strings.TrimSpace(prefix + " " + sub.Name)
		out = append(out, strings.Join(append([]string{p}, sub.Aliases...), "|"))
		out = append(out, paths(sub, p)...)
	}

	return out
}

func flag(t *testing.T, c *spec.Command, name string) spec.Flag {
	t.Helper()

	for _, f := range c.Flags {
		if slices.Contains(f.Names, name) {
			return f
		}
	}

	t.Fatalf("flag %s not found", name)

	return spec.Flag{}
}

func TestWalkKeepsOnlyCommandsWithTheirOwnHelp(t *testing.T) {
	t.Parallel()

	// the others get the root help
	want := []string{
		"agents", "auth", "install", "mcp", "mcp add", "plugin|plugins", "plugin install|i", "plugin marketplace",
	}

	if got := paths(walk(t), ""); !slices.Equal(got, want) {
		t.Errorf("got commands\n%q\nwant\n%q", got, want)
	}
}

func TestWalkLeavesOutFailedCommands(t *testing.T) {
	t.Parallel()

	want := []string{"agents", "auth", "install", "plugin|plugins", "plugin install|i", "plugin marketplace"}

	if got := paths(walk(t, "mcp"), ""); !slices.Equal(got, want) {
		t.Errorf("got commands\n%q\nwant\n%q", got, want)
	}
}

func TestWalkNeedsTheRoot(t *testing.T) {
	t.Parallel()

	if _, err := spec.Walk(t.Context(), fixtureRunner(t, "")); err == nil {
		t.Error("expected an error when the root help fails")
	}
}

func TestFlagInvariants(t *testing.T) {
	t.Parallel()

	var check func(cmd *spec.Command)

	check = func(cmd *spec.Command) {
		for _, flag := range cmd.Flags {
			for _, name := range flag.Names {
				if !strings.HasPrefix(name, "-") || strings.Contains(name, " ") {
					t.Errorf("%s: bad flag name %q", cmd.Name, name)
				}
			}

			if flag.Desc == "" {
				t.Errorf("%s: %s has no description", cmd.Name, flag.Long())
			}

			if slices.Contains(flag.Choices, "default") || slices.Contains(flag.Choices, "preset") {
				t.Errorf("%s: %s choices contain a commander key: %q", cmd.Name, flag.Long(), flag.Choices)
			}
		}

		for _, sub := range cmd.Commands {
			check(sub)
		}
	}

	check(walk(t))
}

func TestChoices(t *testing.T) {
	t.Parallel()

	root := walk(t)

	got := map[string][]string{}

	var collect func(cmd *spec.Command, prefix string)

	collect = func(cmd *spec.Command, prefix string) {
		for _, flag := range cmd.Flags {
			if flag.Choices != nil {
				got[strings.TrimSpace(prefix+" "+flag.Long())] = flag.Choices
			}
		}

		for _, sub := range cmd.Commands {
			collect(sub, prefix+" "+sub.Name)
		}
	}

	collect(root, "")

	want := map[string][]string{
		// (choices: ...) with the default: and preset: keys
		"--input-format":           {"text", "stream-json"},
		"--output-format":          {"text", "json", "stream-json"},
		"--permission-mode":        {"acceptEdits", "auto", "bypassPermissions", "manual", "dontAsk", "plan"},
		"--permission-prompts":     {"host", "none"},
		"--prompt-suggestions":     {"true", "false", "1", "0", "yes", "no", "on", "off"},
		"--system-prompt-snapshot": {"on", "off"},
		// prose lists
		"--effort":                 {"low", "medium", "high", "xhigh", "max"},
		"--setting-sources":        {"user", "project", "local"},
		"agents --setting-sources": {"user", "project", "local"},
		"mcp add --scope":          {"local", "user", "project"},
		"mcp add --transport":      {"stdio", "sse", "http"},
	}

	for name, values := range want {
		if !slices.Equal(got[name], values) {
			t.Errorf("%s: got choices %q, want %q", name, got[name], values)
		}
	}

	for name := range got {
		if _, ok := want[name]; !ok {
			t.Errorf("%s: unexpected choices %q", name, got[name])
		}
	}
}

func TestWrappedFlagNames(t *testing.T) {
	t.Parallel()

	root := walk(t)

	// a name too wide for its column stands alone
	tools := flag(t, root, "--allowed-tools")
	if !slices.Equal(tools.Names, []string{"--allowedTools", "--allowed-tools"}) || tools.Arg != "<tools...>" ||
		!strings.HasPrefix(tools.Desc, "Comma or space-separated list") {
		t.Errorf("got %+v", tools)
	}

	exclude := flag(t, root, "--exclude-dynamic-system-prompt-sections")
	if len(exclude.Names) != 1 || exclude.Arg != "" || !strings.HasPrefix(exclude.Desc, "Move per-machine sections") {
		t.Errorf("got %+v", exclude)
	}
}

func TestFlagArgs(t *testing.T) {
	t.Parallel()

	root := walk(t)

	for _, tc := range []struct {
		name               string
		arg                string
		optional, variadic bool
	}{
		{"--resume", "[value]", true, false},
		{"--add-dir", "<directories...>", false, true},
		{"--model", "<model>", false, false},
		{"--print", "", false, false},
	} {
		f := flag(t, root, tc.name)
		if f.Arg != tc.arg || f.Optional() != tc.optional || f.Variadic() != tc.variadic {
			t.Errorf("%s: got arg %q optional %v variadic %v", tc.name, f.Arg, f.Optional(), f.Variadic())
		}
	}

	if r := flag(t, root, "-r"); !slices.Equal(r.Names, []string{"-r", "--resume"}) {
		t.Errorf("got names %q", r.Names)
	}
}

func TestCommandRows(t *testing.T) {
	t.Parallel()

	root := walk(t)

	mcp := root.Find("mcp")

	// not the Examples of `mcp add`, not the help command
	names := make([]string, 0, len(mcp.Commands))
	for _, cmd := range mcp.Commands {
		names = append(names, cmd.Name)
	}

	if want := []string{"add"}; !slices.Equal(names, want) {
		t.Errorf("got mcp commands %q, want %q", names, want)
	}

	if add := mcp.Find("add"); add.Desc != "Add an MCP server to Claude Code." {
		t.Errorf("got %+v", add)
	}

	if root.Find("plugins") != root.Find("plugin") || root.Find("plugin").Find("i") == nil {
		t.Error("aliases do not resolve")
	}
}

func TestExecKillsAHangingRun(t *testing.T) {
	t.Parallel()

	// as `claude remote-control --help` does in some environments
	bin := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho 'Usage: claude'\nexec sleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}

	start := time.Now()

	_, err := spec.Exec(bin, 200*time.Millisecond)(t.Context(), "--help")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("got error %v, want a deadline error", err)
	}

	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("run took %s", elapsed)
	}
}

func TestLoadAsksClaudeOnlyWithoutAVersion(t *testing.T) {
	t.Parallel()

	var (
		version  = "2.1.282"
		walks    atomic.Int32
		versions atomic.Int32
		base     = fixtureRunner(t)
	)

	run := func(ctx context.Context, args ...string) ([]byte, error) {
		switch {
		case slices.Equal(args, []string{"--version"}):
			versions.Add(1)

			return []byte(version + " (Claude Code)\n"), nil
		case slices.Equal(args, []string{"--help"}):
			walks.Add(1)
		}

		return base(ctx, args...)
	}

	path := filepath.Join(t.TempDir(), "claude-complete", "spec.json")

	for _, step := range []struct {
		name            string
		known, version  string // known is the version read from the path, empty when it cannot be
		refresh         bool
		walks, versions int32
	}{
		{"first completion", "2.1.282", "2.1.282", false, 1, 0},
		{"version read from the path: no process started", "2.1.282", "2.1.282", false, 1, 0},
		{"updated", "2.1.283", "2.1.283", false, 2, 0},
		{"version unknown: claude is asked", "", "2.1.283", false, 2, 1},
		{"version unknown and updated", "", "2.1.284", false, 3, 2},
		{"refresh", "2.1.284", "2.1.284", true, 4, 2},
	} {
		version = step.version

		loaded, err := spec.Load(t.Context(), run, path, step.known, step.refresh)
		if err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}

		if loaded.Version != step.version || loaded.Root.Find("mcp") == nil {
			t.Errorf("%s: got version %q and root %+v", step.name, loaded.Version, loaded.Root)
		}

		if got, want := [2]int32{walks.Load(), versions.Load()}, [2]int32{step.walks, step.versions}; got != want {
			t.Errorf("%s: got %d walks and %d version checks, want %v", step.name, got[0], got[1], want)
		}
	}
}
