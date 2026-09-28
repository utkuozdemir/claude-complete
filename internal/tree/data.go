package tree

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"go/version"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/carapace-sh/carapace"
)

// A source that cannot be read means no candidates, so lookup errors are dropped.

const (
	newestSessions = 20       // offered for an empty --resume
	tailSize       = 64 << 10 // read from the end of a transcript, where the latest records are
)

// sessionRe matches the name of a transcript, not of e.g. `<id>.orphaned-<n>-<hash>.jsonl`.
var sessionRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.jsonl$`)

// claudeDir returns the CLI data directory, `CLAUDE_CONFIG_DIR` or `~/.claude`.
func claudeDir() string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir
	}

	home, _ := os.UserHomeDir()

	return filepath.Join(home, ".claude")
}

// claudeJSON returns the CLI global configuration file, in `CLAUDE_CONFIG_DIR` or `~/.claude.json`.
func claudeJSON() string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, ".claude.json")
	}

	home, _ := os.UserHomeDir()

	return filepath.Join(home, ".claude.json")
}

// projectPaths returns dir as entered and resolved, the one the CLI records, e.g., `/private/tmp` for `/tmp`.
func projectPaths(dir string) []string {
	if resolved, err := filepath.EvalSymlinks(dir); err == nil && resolved != dir {
		return []string{dir, resolved}
	}

	return []string{dir}
}

// projectDir returns the name of the transcript directory of dir, as the CLI generates it.
func projectDir(dir string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			return r
		}

		return '-'
	}, dir)
}

// session represents one transcript of the current project.
type session struct {
	id      string
	modTime time.Time
	size    int64
	path    string
}

// sessionsOf returns the transcripts of the project at dir, newest first.
func sessionsOf(dir string) []session {
	var out []session

	for _, path := range projectPaths(dir) {
		paths, _ := filepath.Glob(filepath.Join(claudeDir(), "projects", projectDir(path), "*.jsonl"))

		for _, path := range paths {
			info, err := os.Stat(path)
			if err != nil || !sessionRe.MatchString(filepath.Base(path)) {
				continue
			}

			id := strings.TrimSuffix(filepath.Base(path), ".jsonl")
			out = append(out, session{id, info.ModTime(), info.Size(), path})
		}
	}

	slices.SortFunc(out, func(a, b session) int { return b.modTime.Compare(a.modTime) })

	return out
}

// record represents the part of a transcript line the completions read.
type record struct {
	Type         string `json:"type"`
	CustomTitle  string `json:"customTitle"`
	AITitle      string `json:"aiTitle"`
	PRNumber     int    `json:"prNumber"`
	PRRepository string `json:"prRepository"`
}

// records returns the records of the given types from the last tailSize bytes of the transcript, read into
// buf with a single call.
func (sess session) records(buf []byte, types ...string) []record {
	file, err := os.Open(sess.path)
	if err != nil {
		return nil
	}

	defer file.Close()

	window := min(sess.size, tailSize)

	read, err := file.ReadAt(buf[:window], sess.size-window)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil
	}

	needles := make([][]byte, len(types))
	for idx, typ := range types {
		needles[idx] = []byte(`"type":"` + typ + `"`)
	}

	var out []record

	for line := range bytes.Lines(buf[:read]) {
		// skips most lines, the messages, and the first one if the window cuts it
		if !slices.ContainsFunc(needles, func(needle []byte) bool { return bytes.Contains(line, needle) }) {
			continue
		}

		var rec record
		if json.Unmarshal(line, &rec) == nil && slices.Contains(types, rec.Type) {
			out = append(out, rec)
		}
	}

	return out
}

// sessions completes --resume with the session titles, which the CLI accepts in place of the ids, and the
// ids of the sessions without a title.
func sessions(ctx carapace.Context) carapace.Action {
	var values []string

	seen := map[string]bool{}
	buf := make([]byte, tailSize)

	for idx, sess := range sessionsOf(ctx.Dir) {
		if ctx.Value == "" && idx >= newestSessions {
			break
		}

		value, desc := sess.id, ""

		for _, rec := range sess.records(buf, "custom-title", "ai-title") {
			switch rec.Type {
			case "custom-title":
				value = rec.CustomTitle
			case "ai-title":
				desc = rec.AITitle
			}
		}

		if seen[value] { // carapace filters by the typed prefix
			continue
		}

		seen[value] = true
		values = append(values, value, desc)
	}

	return carapace.ActionValuesDescribed(values...)
}

// pullRequests completes --from-pr with the pull requests linked to the sessions of the project.
func pullRequests(ctx carapace.Context) carapace.Action {
	var values []string

	seen := map[int]bool{}
	buf := make([]byte, tailSize)

	for _, sess := range sessionsOf(ctx.Dir) {
		for _, rec := range sess.records(buf, "pr-link") {
			if rec.PRNumber > 0 && !seen[rec.PRNumber] {
				seen[rec.PRNumber] = true
				values = append(values, strconv.Itoa(rec.PRNumber), rec.PRRepository)
			}
		}
	}

	return carapace.ActionValuesDescribed(values...)
}

// agents completes --agent with the agents of the user and of the project.
func agents(ctx carapace.Context) carapace.Action {
	var values []string

	for _, dir := range []string{filepath.Join(claudeDir(), "agents"), filepath.Join(ctx.Dir, ".claude", "agents")} {
		paths, _ := filepath.Glob(filepath.Join(dir, "*.md"))

		for _, path := range paths {
			name, desc := frontMatter(path)
			if name == "" {
				name = strings.TrimSuffix(filepath.Base(path), ".md")
			}

			values = append(values, name, desc)
		}
	}

	return carapace.ActionValuesDescribed(values...)
}

// frontMatter returns the name and description fields of a markdown file's front matter.
func frontMatter(path string) (string, string) {
	file, err := os.Open(path)
	if err != nil {
		return "", ""
	}

	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "---" {
		return "", ""
	}

	var name, desc string

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}

		key, value, _ := strings.Cut(line, ":")
		value = strings.Trim(strings.TrimSpace(value), `"'`)

		switch key {
		case "name":
			name = value
		case "description":
			desc = value
		}
	}

	return name, desc
}

// object reads a JSON object, to be decoded member by member, so that an entry in an unexpected shape costs
// only itself.
func object(path string) map[string]json.RawMessage {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var obj map[string]json.RawMessage
	if json.Unmarshal(data, &obj) != nil {
		return nil
	}

	return obj
}

// member decodes the member key of obj into v and reports whether it had that shape.
func member(obj map[string]json.RawMessage, key string, v any) bool {
	raw, ok := obj[key]

	return ok && json.Unmarshal(raw, v) == nil
}

// mcpServers completes the MCP servers of the user and of the project. With remoteOnly, the stdio servers
// are left out, which cannot be logged in to.
func mcpServers(remoteOnly bool) carapace.CompletionCallback {
	return func(ctx carapace.Context) carapace.Action {
		var values []string

		add := func(obj map[string]json.RawMessage) {
			var servers map[string]json.RawMessage
			if !member(obj, "mcpServers", &servers) {
				return
			}

			for _, name := range slices.Sorted(maps.Keys(servers)) {
				var server struct {
					Type    string `json:"type"`
					Command string `json:"command"`
				}

				_ = json.Unmarshal(servers[name], &server) // still offered, without a description

				kind := server.Type
				if kind == "" && server.Command != "" {
					kind = "stdio"
				}

				if !remoteOnly || kind != "stdio" {
					values = append(values, name, kind)
				}
			}
		}

		user := object(claudeJSON())
		add(user)

		var projects map[string]json.RawMessage
		if member(user, "projects", &projects) {
			for _, path := range projectPaths(ctx.Dir) {
				var project map[string]json.RawMessage
				if member(projects, path, &project) {
					add(project)
				}
			}
		}

		add(object(filepath.Join(ctx.Dir, ".mcp.json")))

		return carapace.ActionValuesDescribed(values...)
	}
}

// modelOptions completes the model options the CLI cached for the account.
func modelOptions(carapace.Context) carapace.Action {
	var options []json.RawMessage

	member(object(claudeJSON()), "additionalModelOptionsCache", &options)

	values := make([]string, 0, 2*len(options))

	for _, raw := range options {
		var option struct {
			Value       string `json:"value"`
			Description string `json:"description"`
		}

		if json.Unmarshal(raw, &option) == nil && option.Value != "" {
			values = append(values, option.Value, option.Description)
		}
	}

	return carapace.ActionValuesDescribed(values...)
}

// modelIDRe matches a model id the CLI accepts in its catalog.
var modelIDRe = regexp.MustCompile(`^[a-z0-9._:@/\[\]-]{1,200}$`)

// catalogModels completes the models /model offers, from the catalog the CLI cached for the account and the
// CLI surface (`cc`). The masked models and the ones needing a newer claude are left out.
func catalogModels(installed string) carapace.CompletionCallback {
	return func(carapace.Context) carapace.Action {
		var account struct {
			OrganizationUUID string `json:"organizationUuid"`
			AccountUUID      string `json:"accountUuid"`
		}

		if !member(object(claudeJSON()), "oauthAccount", &account) ||
			account.OrganizationUUID == "" || account.AccountUUID == "" {
			return carapace.ActionValues()
		}

		dir := filepath.Join(claudeDir(), "cache", "model-catalog")
		scope := sha256.Sum256([]byte(account.AccountUUID))
		name := account.OrganizationUUID + "-" + hex.EncodeToString(scope[:])[:12] + "-cc.json"

		var (
			masked []string
			doc    struct {
				Config struct {
					Models []json.RawMessage `json:"models"`
				} `json:"config"`
			}
		)

		member(object(filepath.Join(dir, "masked-ids.json")), "ids", &masked)
		member(object(filepath.Join(dir, name)), "catalog", &doc)

		values := make([]string, 0, 2*len(doc.Config.Models))

		for _, raw := range doc.Config.Models {
			var model struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				Description string `json:"description"`
				MinVersion  string `json:"min_claude_code_version"` //nolint:tagliatelle // the CLI's key
			}

			if json.Unmarshal(raw, &model) != nil || !modelIDRe.MatchString(model.ID) ||
				slices.Contains(masked, model.ID) || !atLeast(installed, model.MinVersion) {
				continue
			}

			desc := model.Name
			if model.Description != "" {
				desc += " · " + model.Description
			}

			values = append(values, model.ID, desc)
		}

		return carapace.ActionValuesDescribed(values...)
	}
}

// atLeast reports whether installed is at least minimum. go/version compares them as it compares Go
// toolchains, and a minimum it cannot read is never met.
func atLeast(installed, minimum string) bool {
	return minimum == "" || version.IsValid("go"+minimum) && version.Compare("go"+installed, "go"+minimum) >= 0
}

// projects completes `claude project purge` with the projects the CLI keeps state for.
func projects(carapace.Context) carapace.Action {
	var projects map[string]json.RawMessage

	member(object(claudeJSON()), "projects", &projects)

	return carapace.ActionValues(slices.Sorted(maps.Keys(projects))...)
}

// installedPlugins completes the installed plugins, as `name@marketplace`.
func installedPlugins(carapace.Context) carapace.Action {
	var plugins map[string]json.RawMessage

	member(object(filepath.Join(claudeDir(), "plugins", "installed_plugins.json")), "plugins", &plugins)

	values := make([]string, 0, 2*len(plugins))

	for _, id := range slices.Sorted(maps.Keys(plugins)) {
		var installs []struct {
			Version string `json:"version"`
			Scope   string `json:"scope"`
		}

		desc := ""
		if json.Unmarshal(plugins[id], &installs) == nil && len(installs) > 0 {
			desc = strings.TrimSpace(installs[0].Version + " (" + installs[0].Scope + ")")
		}

		values = append(values, id, desc)
	}

	return carapace.ActionValuesDescribed(values...)
}

// marketplace represents a plugin marketplace the CLI knows of.
type marketplace struct {
	Source struct {
		Repo string `json:"repo"`
		URL  string `json:"url"`
	} `json:"source"`
	InstallLocation string `json:"installLocation"`
}

// knownMarketplaces returns the registered marketplaces by name.
func knownMarketplaces() map[string]marketplace {
	obj := object(filepath.Join(claudeDir(), "plugins", "known_marketplaces.json"))
	known := make(map[string]marketplace, len(obj))

	for name := range obj {
		var market marketplace

		_ = json.Unmarshal(obj[name], &market) // still known by its name

		if market.InstallLocation == "" {
			market.InstallLocation = filepath.Join(claudeDir(), "plugins", "marketplaces", name)
		}

		known[name] = market
	}

	return known
}

// marketplaces completes the names of the registered marketplaces.
func marketplaces(carapace.Context) carapace.Action {
	known := knownMarketplaces()
	values := make([]string, 0, 2*len(known))

	for _, name := range slices.Sorted(maps.Keys(known)) {
		source := known[name].Source.Repo
		if source == "" {
			source = known[name].Source.URL
		}

		values = append(values, name, source)
	}

	return carapace.ActionValuesDescribed(values...)
}

// marketplacePlugins completes `claude plugin install` from the marketplace catalogs, as `name@marketplace`.
func marketplacePlugins(carapace.Context) carapace.Action {
	var values []string

	known := knownMarketplaces()

	for _, name := range slices.Sorted(maps.Keys(known)) {
		var plugins []json.RawMessage

		member(
			object(filepath.Join(known[name].InstallLocation, ".claude-plugin", "marketplace.json")),
			"plugins",
			&plugins,
		)

		for _, raw := range plugins {
			var plugin struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			}

			if json.Unmarshal(raw, &plugin) == nil && plugin.Name != "" {
				values = append(values, plugin.Name+"@"+name, plugin.Description)
			}
		}
	}

	return carapace.ActionValuesDescribed(values...)
}

// jobs completes the ids of the background sessions.
func jobs(carapace.Context) carapace.Action {
	paths, _ := filepath.Glob(filepath.Join(claudeDir(), "jobs", "*", "state.json"))

	values := make([]string, 0, 2*len(paths))

	for _, path := range paths {
		var state struct {
			Name  string `json:"name"`
			State string `json:"state"`
		}

		desc := ""
		if data, err := os.ReadFile(path); err == nil && json.Unmarshal(data, &state) == nil {
			desc = strings.TrimSpace(state.Name + " (" + state.State + ")")
		}

		values = append(values, filepath.Base(filepath.Dir(path)), desc)
	}

	return carapace.ActionValuesDescribed(values...)
}
