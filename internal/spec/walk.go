package spec

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Runner runs the claude binary with the given arguments and returns its standard output.
type Runner func(ctx context.Context, args ...string) ([]byte, error)

// Exec returns a Runner for the binary bin, which kills every run at the timeout.
func Exec(bin string, timeout time.Duration) Runner {
	return func(ctx context.Context, args ...string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		// claude on Linux exits before a pipe takes all of its output, so it writes to a file
		out, err := os.CreateTemp("", "claude-complete-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create the output file: %w", err)
		}

		defer os.Remove(out.Name())
		defer out.Close()

		cmd := exec.CommandContext(ctx, bin, args...)
		cmd.Stdout = out

		err = cmd.Run()

		if ctx.Err() != nil {
			return nil, fmt.Errorf("claude %s: %w", strings.Join(args, " "), ctx.Err())
		}

		if err != nil {
			return nil, fmt.Errorf("claude %s: %w", strings.Join(args, " "), err)
		}

		return os.ReadFile(out.Name())
	}
}

// Version returns the version of the claude binary, e.g., "2.1.282".
func Version(ctx context.Context, run Runner) (string, error) {
	out, err := run(ctx, "--version")
	if err != nil {
		return "", fmt.Errorf("failed to get the claude version: %w", err)
	}

	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return "", errors.New("claude printed no version")
	}

	return fields[0], nil
}

// workers is how many help commands run at once.
const workers = 8

// node is a command whose help is not read yet.
type node struct {
	parent *Command
	stub   *Command
	path   []string
}

// Walk reads the command tree by running `claude <path> --help` for every command, level by level.
//
// A command whose help fails, times out or belongs to another command is left out. Only the root is
// required.
func Walk(ctx context.Context, run Runner) (*Command, error) {
	out, err := run(ctx, "--help")
	if err != nil {
		return nil, fmt.Errorf("failed to read the claude help: %w", err)
	}

	rootPage := parse(string(out))
	if !usageMatches(rootPage.usage, nil) {
		return nil, fmt.Errorf("unexpected claude help, usage is %q", rootPage.usage)
	}

	root := &Command{Desc: rootPage.desc, Flags: rootPage.flags}

	level := children(root, nil, rootPage.commands)

	for len(level) > 0 {
		level = walkLevel(ctx, run, level)
	}

	return root, nil
}

// walkLevel reads the help of every node, adds the found ones to their parents and returns the next level.
func walkLevel(ctx context.Context, run Runner, level []node) []node {
	pages := make([]*page, len(level))
	sem := make(chan struct{}, workers)

	var wg sync.WaitGroup

	for idx, current := range level {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()

			out, err := run(ctx, append(current.path, "--help")...)
			if err != nil {
				return
			}

			// `claude <unknown> --help` exits with 0 and prints the root help
			if help := parse(string(out)); usageMatches(help.usage, current.path) {
				pages[idx] = &help
			}
		})
	}

	wg.Wait()

	var next []node

	for idx, current := range level {
		if pages[idx] == nil {
			continue
		}

		current.stub.Flags = pages[idx].flags
		current.parent.Commands = append(current.parent.Commands, current.stub)

		next = append(next, children(current.stub, current.path, pages[idx].commands)...)
	}

	return next
}

func children(parent *Command, path []string, stubs []*Command) []node {
	nodes := make([]node, 0, len(stubs))

	for _, stub := range stubs {
		nodes = append(nodes, node{parent, stub, append(append([]string(nil), path...), stub.Name)})
	}

	return nodes
}

// usageMatches reports whether a usage line belongs to the command at path, e.g.,
// "claude plugin install|i [options] <plugin>".
func usageMatches(usage string, path []string) bool {
	want := strings.Join(append([]string{"claude"}, path...), " ")

	rest, ok := strings.CutPrefix(usage, want)

	return ok && (rest == "" || rest[0] == ' ' || rest[0] == '|')
}
