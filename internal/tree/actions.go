package tree

import (
	"regexp"
	"slices"
	"strings"

	"github.com/carapace-sh/carapace"

	"github.com/utkuozdemir/claude-complete/internal/spec"
)

// quotedRe matches a model name in the --model help, e.g., 'opus'.
var quotedRe = regexp.MustCompile(`'([a-z][\w.-]*)'`)

// completer contains what the actions need from the spec as a whole.
type completer struct {
	version string
	models  []string
	// the listed values by flag name and placeholder, which a flag listing none borrows, e.g.,
	// `agents --effort <level>` from `--effort <level>`
	choices map[string][]string
}

func newCompleter(tree *spec.Spec) completer {
	root := tree.Root
	comp := completer{version: tree.Version, choices: map[string][]string{}}
	conflicting := map[string]bool{}

	var collect func(cmd *spec.Command)

	collect = func(cmd *spec.Command) {
		for _, flag := range cmd.Flags {
			if cmd == root && flag.Long() == "--model" {
				for _, match := range quotedRe.FindAllStringSubmatch(flag.Desc, -1) {
					comp.models = append(comp.models, match[1])
				}
			}

			if len(flag.Choices) == 0 {
				continue
			}

			key := flag.Long() + " " + flag.Arg
			if seen, ok := comp.choices[key]; ok && !slices.Equal(seen, flag.Choices) {
				conflicting[key] = true
			}

			comp.choices[key] = flag.Choices
		}

		for _, sub := range cmd.Commands {
			collect(sub)
		}
	}

	collect(root)

	for key := range conflicting { // ambiguous
		delete(comp.choices, key)
	}

	return comp
}

// action returns how to complete the value of the flag: from real data, from the listed choices, or from the
// filesystem for a path. Any other value is not completed, e.g., `--json-schema <schema>`.
func (comp completer) action(flag spec.Flag) carapace.Action {
	values, ok := comp.value(flag)
	if !ok {
		return carapace.ActionValues()
	}

	if strings.Contains(strings.ToLower(flag.Desc), "comma-separated") {
		return carapace.ActionMultiParts(",", func(ctx carapace.Context) carapace.Action {
			return values.Filter(ctx.Parts...)
		})
	}

	return values
}

func (comp completer) value(flag spec.Flag) (carapace.Action, bool) {
	switch flag.Long() {
	case "--resume":
		return safe(sessions), true
	case "--from-pr":
		return safe(pullRequests), true
	case "--agent":
		return safe(agents), true
	case "--model", "--fallback-model":
		return carapace.Batch(
			carapace.ActionValues(comp.models...),
			safe(modelOptions),
			safe(catalogModels(comp.version)),
		).ToA(), true
	case "--plugin-dir":
		return carapace.ActionFiles(".zip"), true
	}

	choices := flag.Choices
	if len(choices) == 0 {
		choices = comp.choices[flag.Long()+" "+flag.Arg]
	}

	if len(choices) > 0 {
		return carapace.ActionValues(choices...), true
	}

	arg := strings.ToLower(flag.Arg)

	switch {
	case strings.Contains(arg, "dir"), flag.Long() == "--cwd":
		return carapace.ActionDirectories(), true
	case strings.Contains(arg, "path"), strings.Contains(arg, "file"), strings.Contains(arg, "config"):
		return carapace.ActionFiles(), true
	}

	return carapace.Action{}, false
}

// positional returns how to complete the arguments of the command at path, if they come from real data.
func positional(path string) (carapace.Action, bool) {
	switch path {
	case "mcp get", "mcp remove":
		return safe(mcpServers(false)), true
	case "mcp login", "mcp logout":
		return safe(mcpServers(true)), true
	case "attach", "logs", "stop", "respawn", "rm":
		return safe(jobs), true
	case "project purge":
		return safe(projects), true
	case "plugin enable", "plugin disable", "plugin uninstall", "plugin update", "plugin details":
		return safe(installedPlugins), true
	case "plugin install":
		return safe(marketplacePlugins), true
	case "plugin marketplace remove", "plugin marketplace update":
		return safe(marketplaces), true
	}

	return carapace.Action{}, false
}

// safe runs a data source, and returns no candidates if it panics. carapace runs it in a goroutine, where
// the recover in main does not reach.
func safe(source carapace.CompletionCallback) carapace.Action {
	return carapace.ActionCallback(func(ctx carapace.Context) carapace.Action {
		action := carapace.ActionValues()

		func() {
			defer func() { _ = recover() }()

			action = source(ctx)
		}()

		return action
	})
}
