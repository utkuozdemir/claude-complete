# claude-complete

Shell completion for the [Claude Code](https://claude.com/claude-code) CLI.

Claude Code has no shell completion of its own.
`claude-complete` reads the commands and flags from the `claude` you have installed, so they match your version after every update.
It also completes values from the data Claude Code keeps on your machine:

- `claude --resume <TAB>`: the sessions of the project, by title, with a description of what they were about
- `claude --from-pr <TAB>`: the pull requests linked to those sessions
- `claude --model <TAB>`: the model aliases and the models of your account, as `/model` offers them
- `claude --agent <TAB>`: your agents
- `claude mcp get <TAB>`: your MCP servers
- `claude stop <TAB>`: your background sessions
- `claude plugin install <TAB>`: the plugins of the marketplaces you added
- `claude plugin enable <TAB>`, `disable`, `update` and so on: the plugins you installed
- `claude project purge <TAB>`: the projects Claude Code keeps state for
- `--effort`, `--permission-mode` and the other flags with a fixed set of values: those values
- flags that take a path: files or directories

Both `--flag value` and `--flag=value` work.

## Install

With [Homebrew](https://brew.sh), on macOS and Linux:

```sh
brew install utkuozdemir/tap/claude-complete
```

Or with the package of your Linux distribution, from the [releases](https://github.com/utkuozdemir/claude-complete/releases):

```sh
sudo apt install ./claude-complete_*_linux_amd64.deb      # Debian, Ubuntu
sudo dnf install ./claude-complete_*_linux_amd64.rpm      # Fedora, RHEL
sudo apk add --allow-untrusted ./claude-complete_*_linux_amd64.apk   # Alpine
sudo pacman -U ./claude-complete_*_linux_amd64.pkg.tar.zst           # Arch
```

Both install the completion scripts where bash, zsh and fish load them, so completion works in the next shell you open.
With Homebrew, zsh and bash need Homebrew's completions set up, as described in [Homebrew Shell Completion](https://docs.brew.sh/Shell-Completion).

Or with Go, and then a line in your shell's startup file:

```sh
go install github.com/utkuozdemir/claude-complete@latest
```

| Shell | Add to your shell's startup file |
| --- | --- |
| zsh | `source <(claude-complete _carapace zsh)`, after `compinit` (after oh-my-zsh, if you use it) |
| bash 4.4 or newer | `source <(claude-complete _carapace bash)` |
| fish | `claude-complete _carapace fish \| source` |

macOS ships bash 3.2, which is too old. Use `brew install bash`, or zsh.

The completions come from [carapace](https://carapace.sh), which also supports nushell, PowerShell, elvish, xonsh and others.
Those are not tested here.

## How it works

The first completion after a Claude Code update runs `claude <command> --help` for every command.
This takes about a second, and the result is cached under `~/.cache/claude-complete`.
Every completion after that reads the cache, and starts no other process.

`claude-complete spec` prints what it read.

An update is noticed because the `claude` on your `PATH` becomes a different file.
If yours is a wrapper script that never changes, run `claude-complete spec --refresh` after you update Claude Code.

## License

[MIT](LICENSE)
