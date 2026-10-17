// Command claude-complete implements shell completion for the Claude Code CLI.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"
	"time"

	"github.com/carapace-sh/carapace"
	"github.com/spf13/cobra"

	"github.com/utkuozdemir/claude-complete/internal/spec"
	"github.com/utkuozdemir/claude-complete/internal/tree"
)

// helpTimeout is how long a run of claude may take.
const helpTimeout = 2 * time.Second

// version is set at release time.
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "claude-complete:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 0 && args[0] == "_carapace" {
		complete(args)

		return nil
	}

	cmd := &cobra.Command{
		Use:   "claude-complete",
		Short: "Shell completion for the Claude Code CLI",
		Long: "Shell completion for the Claude Code CLI.\n\n" +
			"Load it in your shell with one of:\n\n" +
			"  source <(claude-complete _carapace zsh)    # zsh, after compinit\n" +
			"  source <(claude-complete _carapace bash)   # bash 4.4 or newer\n" +
			"  claude-complete _carapace fish | source    # fish",
		Version:      buildVersion(),
		SilenceUsage: true,
	}
	cmd.CompletionOptions.DisableDefaultCmd = true

	var refresh bool

	specCmd := &cobra.Command{
		Use:   "spec",
		Short: "Print the command tree read from the installed claude",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			loaded, err := load(cmd.Context(), refresh)
			if err != nil {
				return err
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			enc.SetEscapeHTML(false) // print <directories...> as is

			if err = enc.Encode(loaded); err != nil {
				return fmt.Errorf("failed to print the spec: %w", err)
			}

			return nil
		},
	}
	specCmd.Flags().BoolVar(&refresh, "refresh", false, "read the help again, even if the cached tree is current")

	cmd.AddCommand(specCmd)
	cmd.SetArgs(args)

	if err := cmd.Execute(); err != nil {
		return fmt.Errorf("%s: %w", strings.Join(args, " "), err)
	}

	return nil
}

// complete prints the shell snippet, or the completions of a command line. A failure means no completions.
func complete(args []string) {
	defer func() { _ = recover() }()

	root, execArgs := tree.Bare(), args

	// the snippet needs no claude, which may not be on PATH at shell startup yet
	if len(args) > 2 {
		if loaded, err := load(context.Background(), false); err == nil {
			root, execArgs = tree.New(loaded, args)
		}
	}

	carapace.Gen(root).Standalone()
	root.SetArgs(execArgs)

	_ = root.Execute()
}

func load(ctx context.Context, refresh bool) (*spec.Spec, error) {
	path, err := spec.CachePath()
	if err != nil {
		return nil, fmt.Errorf("failed to load the spec: %w", err)
	}

	// reading the version from the installed files saves starting claude on every completion
	var version string
	if bin, err := exec.LookPath("claude"); err == nil {
		version = spec.VersionFromPath(bin)
	}

	loaded, err := spec.Load(ctx, spec.Exec("claude", helpTimeout), path, version, refresh)
	if err != nil {
		return nil, fmt.Errorf("failed to load the spec: %w", err)
	}

	return loaded, nil
}

// buildVersion returns the version set at release time, or else the module version, e.g., for `go install`.
func buildVersion() string {
	if version != "dev" {
		return version
	}

	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}

	return version
}
