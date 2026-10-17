# claude-complete — agent guide

This is the project's knowledge base for humans and agents working in this repository.
Maintain it as you go: when you learn something durable about how this repo works, add it here.
It is not append-only, so fix or delete anything that becomes wrong.

Keep it timeless and project-general: no in-flight work, no individual's local setup.
Prefer pointers over copies, and invariants over inventories.
Flag lists, command lists and version numbers live in the code and in `claude --help`.

`task lint` runs `markdownlint` over every tracked markdown file, this one included.
Keep one sentence per line and a single top-level heading.

`CLAUDE.md` and `GEMINI.md` import this file, so this is the one place to edit.

## What this repo is

Shell completion for the Claude Code CLI, as one Go binary.

Claude Code is built on commander.js, which has no completion support.
The CLI also has no machine-readable description of itself.
So the command tree is read from `claude <path> --help`, on the user's machine, and cached.

Flag values come from the data the CLI keeps on disk:

- sessions and the pull requests linked to them
- MCP servers, agents, background sessions
- plugins, marketplaces, projects
- cached model options and the model catalog

A flag whose help lists no values borrows them from the flag with the same name and placeholder, unless those disagree.
carapace generates the completion script and the output for every shell.

Deliberate non-goals:

- **No baked flag or command lists.** The installed `claude` is the source of truth. Competing projects ship a static list, and it is outdated with every Claude Code release, which is roughly daily.
- **No hidden-command constants.** Commands and flags missing from the help are not completed. Unknown flags are still accepted on the line.
- **No guessing values.** A flag with no local source for its values, e.g., a JSON schema, is not completed. A wrong insertion is worse than none.
- **No shell-specific code.** Quoting, argument scanning and the shell scripts are carapace's job.

## Facts about claude the code relies on

- `claude <unknown> --help` exits with 0 and prints the root help. A command is real only when its `Usage:` line names its own path (`usageMatches` in `internal/spec/walk.go`).
- Some help commands hang. Every run of `claude` has a timeout, and a command that times out is left out of the tree.
- `claude` has been seen on Linux exiting before a pipe took all of its output, which cut a piped help at a multiple of 8 KB. So its output is written to a file (`Exec` in `internal/spec/walk.go`).
- commander prints a flag name too wide for its column alone on its line. So the parser splits the name from the description on the first line of a row only.
- Allowed values appear as a `(choices: ...)` suffix, or as a prose list in the description. A placeholder such as `<auto|tokens>` is not a list of values.
- The CLI keeps its data in `~/.claude` and its global configuration in `~/.claude.json`. `CLAUDE_CONFIG_DIR` moves both (`claudeDir` and `claudeJSON` in `internal/tree/data.go`).
- Transcripts live in `projects/<dir>` under the data directory. `<dir>` is the working directory, with every non-alphanumeric character replaced by a dash.
- The CLI records a directory reached through a symlink by its resolved path, while the shell reports it as entered. So the working directory is looked up both ways (`projectPaths`).
- Session titles are records appended to the transcript, so the latest one is near the end. Only the tail is read (`tailSize`).
- `claude --resume` accepts a title in place of a session id, so titles are inserted.
- The model catalog is cached per organization, account and surface, as `<organization>-<sha256(account)[:12]>-<surface>.json`. The CLI surface is `cc`.

## Behavior contracts

- **The cache is for one `claude` version, and a known install starts no process to check it.** `spec.VersionFromPath` reads the version from where `claude` resolves to:
  - the native installer: `.../claude/versions/<version>`
  - the Homebrew casks: `.../Caskroom/claude-code/<version>/claude`, and `claude-code@latest`
  - For any other layout, e.g., npm or a wrapper script, `claude --version` runs on every completion. That is correct, only slower.
  - Only a version the path itself carries counts. A version from a manifest next to the binary is not guaranteed to match it.
  - Running `--version` on every completion took two thirds of a completion's time with the native installer, so keep the known layouts up to date when an installer changes.
- **Printing the shell snippet never runs claude.** It happens at shell startup, when `claude` may not be on `PATH` yet (`tree.Bare`).
- **A failure never reaches the prompt.** Missing or unreadable data, or a failed walk, means no completions.
  - The CLI files are internal and change shape between releases. So JSON is decoded member by member (`object` and `member`), and an entry in an unexpected shape affects only itself.
  - Every data source runs under `safe`. carapace runs sources in goroutines, where the recover in `main` does not reach.
  - A cached spec that fails `valid` is walked again.
- **The line is read the way commander reads it.** pflag has no optional values (`--resume [value]`) and no variadic ones (`--add-dir <dirs...>`).
  - `internal/tree/args.go` reads the typed words with commander's rules, for every flag of the command reached, and writes each value as `--flag=value`.
  - Without this, a value named like a subcommand, e.g., a session titled mcp, is read as that subcommand.
  - When in doubt, commander's `parseOptions` is the reference. For example, a variadic flag written as `--flag=value` takes that one value only.
- **An unknown flag does not end completion.** Every flag is a string array, and a flag listed without a value still accepts `--flag=value`, e.g., `--tmux=classic`.

## Code map

- `main.go` has the commands: `_carapace` for the shells, `spec` for debugging.
- `internal/spec` reads the help: the parser, the concurrent walker and the cache.
- `internal/tree` builds the cobra tree carapace completes.
  - cobra is only a data structure here. Every command gets a no-op `Run`, since cobra hides commands that cannot run.
  - `actions.go` decides how a flag or an argument is completed, `data.go` reads the sources, `args.go` reads the command line.

Tests:

- `internal/spec` parses help captured from a real `claude` under `testdata`. When the help format changes, capture the fixtures again from a current `claude`, never edit them manually.
- `main_test.go` builds the binary and completes through `_carapace export`, with a cached spec, a fake `claude` and a fabricated `~/.claude`.
  - The words passed are the command line as the shell scripts send them, starting with `claude`.
  - `testdata/spec.json` is the output of `claude-complete spec` for a real version.
- `shell_test.go` loads the snippet in zsh, bash and fish, and completes through each shell. A missing shell is skipped, CI installs all three.
- The live tests walk the real `claude`, with `-tags live`.
  - `internal/spec` checks the help format.
  - `internal/tree` checks that every flag and command in `flagSources` and `commandSources` still exists, and that the `--model` help still quotes model aliases.
  - The `canary` workflow runs them daily against the latest Claude Code from npm. A failure there means Claude Code changed, not this repo.
- Fuzzing covers the two places that read input nobody here controls:
  - `FuzzParse` feeds arbitrary help. No choice may be empty, and the cache must accept what the parser produced.
  - `FuzzLine` feeds arbitrary command lines to the scanner. Its output must not change when scanned again. This cannot tell a wrong reading from a right one, so a scanner fix still needs its case in the end-to-end table.
  - The JSON sources are not fuzzed. The decoding is the standard library's, and the selection rules have table tests with malformed entries.
  - `task test:fuzz` runs the search. Its failures are kept under `testdata/fuzz`, and `task test` runs them again.
  - Keep a fuzz input cheap. Creating a directory per input made the fuzzing stall for a minute on every new input.

Performance:

- A completion takes a process start, the same as for an empty Go program, plus the tree build, plus what its data source reads.
- Measure changes on real completions: `claude-complete _carapace export claude <words>` in a loop, against the real `~/.claude`.
- `task bench` runs `BenchmarkSources` and `BenchmarkLoadCached`. Closing their freshly written files costs more than closing real transcripts, so compare the numbers, do not read them as completion times.
- Never call `tree.New` in a loop in a test. carapace keeps every tree it gets until the process exits, so memory grows without bound and each call gets slower. `export_test.go` exposes the scanner and the data sources for this reason.
- Opening the transcripts costs more than parsing them. Each tail is read with one call. Opening fewer files would need an index of its own.

## Demo

- `task demo:record` re-records `img/demo.gif` from `demo/demo.tape`, in the recorder's image with zsh and `claude` added (`demo/Dockerfile`), against a made-up `~/.claude` (`demo/setup.sh`). The `demo` workflow does the same from the GitHub UI and opens a pull request.
- The tape picks an item by counting Tab presses. That relies on `menu yes select` in `demo/zshrc`, where the first Tab already selects, and on the order of the candidates. A new flag that sorts before `--resume`, for example, moves it, so look at the recording after re-recording.
- A pull request opened with the workflow's token starts no workflows, so the `demo` workflow starts `build` on its branch itself, which `main` requires.

## Verifying changes

`task lint`, then `task test`.
After a Claude Code update that changes the help, `task test:live` checks the parser and the data sources against the installed `claude`.

## Release

`task release` tags the next version from the tip of `main`, and the release workflow builds it with goreleaser.
The release publishes archives, Linux packages and a Homebrew cask in `utkuozdemir/homebrew-tap`, which needs the `PRIVATE_ACCESS_TOKEN` secret.
The packages and the cask install the completion scripts where the shells load them, so no startup file changes are needed.
The binaries are not notarized, and macOS refuses to run a quarantined one, so the cask removes the quarantine in `postflight_steps`. Homebrew 7 warns about `postflight` blocks.
goreleaser generates the scripts at release time, with `claude-complete _carapace <shell>`.
The zsh directory differs by distribution: `/usr/share/zsh/vendor-completions` on Debian, `/usr/share/zsh/site-functions` elsewhere.
CI builds a snapshot release on every change, so a broken release configuration fails before a tag.
The changelog groups come from the conventional commit type.
The binary is installed with `go install`, so `go.mod` must never have a `replace` directive, which makes `go install ...@latest` fail.

## Commit conventions

Conventional commit titles, `type: imperative description`, with a body that explains what changed and why.
Each body paragraph is one line.
Commits have a DCO `Signed-off-by` trailer.
