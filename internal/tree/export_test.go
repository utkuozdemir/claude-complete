package tree

import (
	"github.com/carapace-sh/carapace"

	"github.com/utkuozdemir/claude-complete/internal/spec"
)

// Tests call these instead of New in a loop, since carapace keeps every tree it gets until the process
// exits.

// Normalize returns the typed words of line as the scanner rewrites them.
func Normalize(root *spec.Command, line []string) []string {
	return normalize(root, line).words
}

// Sources returns the data sources by name.
func Sources(installed string) map[string]carapace.CompletionCallback {
	return map[string]carapace.CompletionCallback{
		"sessions":           sessions,
		"pull requests":      pullRequests,
		"mcp servers":        mcpServers(false),
		"plugins to install": marketplacePlugins,
		"projects":           projects,
		"model options":      modelOptions,
		"model catalog":      catalogModels(installed),
	}
}
