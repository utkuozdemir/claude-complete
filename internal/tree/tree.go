// Package tree builds the cobra command tree that carapace completes, from the spec.
package tree

import (
	"strings"

	"github.com/carapace-sh/carapace"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/utkuozdemir/claude-complete/internal/spec"
)

// Bare returns a root with no subcommands or flags, which is enough to print the shell snippet.
func Bare() *cobra.Command {
	root := &cobra.Command{Use: "claude", Run: func(*cobra.Command, []string) {}}
	root.CompletionOptions.DisableDefaultCmd = true

	return root
}

// New returns the tree for the spec, and args, `_carapace <shell> claude <words...>`, rewritten so that pflag
// reads them the way commander does.
func New(tree *spec.Spec, args []string) (*cobra.Command, []string) {
	root := Bare()
	root.Short = tree.Root.Desc

	comp := newCompleter(tree)
	comp.build(root, tree.Root, "")

	line := line(args)
	if line == nil {
		return root, args
	}

	scanned := normalize(tree.Root, line)
	if scanned.slot != nil {
		cmd := root
		for _, name := range scanned.path {
			cmd = find(cmd, name)
		}

		// commander reads the word as the flag's value, while carapace would offer subcommands
		for _, sub := range cmd.Commands() {
			sub.Hidden = true
		}

		carapace.Gen(cmd).PositionalAnyCompletion(comp.action(*scanned.slot))
	}

	return root, append(append(append([]string(nil), args[:3]...), scanned.words...), line[len(line)-1])
}

// line returns the words after `claude`, or nil when args is not a completion request.
func line(args []string) []string {
	if len(args) < 4 || args[0] != "_carapace" {
		return nil
	}

	return args[3:]
}

// find returns the subcommand of cmd called name.
func find(cmd *cobra.Command, name string) *cobra.Command {
	for _, sub := range cmd.Commands() {
		if sub.Name() == name {
			return sub
		}
	}

	return cmd
}

func (comp completer) build(cmd *cobra.Command, node *spec.Command, path string) {
	cmd.Run = func(*cobra.Command, []string) {} // cobra lists only runnable commands
	cmd.FParseErrWhitelist.UnknownFlags = true  // e.g., the unlisted --system-prompt-file

	actions := carapace.ActionMap{}

	for _, flag := range node.Flags {
		for _, long := range define(cmd.Flags(), flag) {
			if flag.Arg != "" {
				actions[long] = comp.action(flag)
			}
		}
	}

	carapace.Gen(cmd).FlagCompletion(actions)

	if pos, ok := positional(path); ok {
		carapace.Gen(cmd).PositionalCompletion(pos)
	}

	for _, child := range node.Commands {
		sub := &cobra.Command{Use: child.Name, Aliases: child.Aliases, Short: child.Desc}
		comp.build(sub, child, strings.TrimSpace(path+" "+child.Name))
		cmd.AddCommand(sub)
	}
}

// define adds the flag to the set, one flag per long name since pflag has no aliases, and returns the long
// names.
//
// Every flag is a string array, so that a repeated flag stays valid. One whose value may be left out,
// every boolean included, takes a value only as --flag=value, e.g., --tmux=classic.
func define(flags *pflag.FlagSet, flag spec.Flag) []string {
	var defined []string

	for _, name := range flag.Names {
		long, ok := strings.CutPrefix(name, "--")
		if !ok || flags.Lookup(long) != nil {
			continue
		}

		short := ""
		if name == flag.Long() {
			short = shorthand(flags, flag)
		}

		flags.StringArrayP(long, short, nil, flag.Desc)

		if flag.Arg == "" || flag.Optional() {
			flags.Lookup(long).NoOptDefVal = " "
		}

		defined = append(defined, long)
	}

	return defined
}

// shorthand returns the short name of the flag, unless the set already has it.
func shorthand(flags *pflag.FlagSet, flag spec.Flag) string {
	name := flag.Names[0]
	if len(name) != 2 || name[0] != '-' || flags.ShorthandLookup(name[1:]) != nil {
		return ""
	}

	return name[1:]
}
