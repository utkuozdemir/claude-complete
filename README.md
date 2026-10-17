# claude-complete

Shell completion for the [Claude Code](https://claude.com/claude-code) CLI.

> [!WARNING]
> **AI disclaimer:** I wrote this completely using AI, but I wouldn't call it slop, personally.

I found out that Claude Code still doesn't offer built-in shell completion.
I checked the other completion projects out there, but didn't like any of them much.
So I decided to build my own, my way.
Hopefully it helps people until Claude Code gets its own completion.

## What it does

`claude-complete` reads the commands and flags from the `claude` you have installed, so they match your version after every update.
It also completes values from the data Claude Code keeps on your machine:

- `claude --resume <TAB>`: the sessions of the project, by title, with a description of what they were about
- `claude --from-pr <TAB>`: the pull requests linked to those sessions
- `claude --model <TAB>`: the model aliases and the models of your account, as `/model` offers them
- `claude --agent <TAB>`: your agents
- `claude mcp get <TAB>`: your MCP servers
- `claude stop <TAB>`: your background sessions
- `claude plugin install <TAB>`: the plugins of the marketplaces you added
- `claude plugin enable <TAB>`, `disable`, `configure`, `update` and so on: the plugins you installed
- `claude purge <TAB>`: the projects Claude Code keeps state for
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
On Linux, bash loads them through the `bash-completion` package, which most distributions install by default.
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
Every completion after that reads the cache.

An update is noticed by the version of the installed `claude`.
It is read from where `claude` is installed, for the native installer and Homebrew.
Otherwise, e.g., for npm or a wrapper script, it is asked from `claude --version` on every completion.

`claude-complete spec` prints what it read, and `claude-complete spec --refresh` reads the help again.

## License

[MIT](LICENSE)
