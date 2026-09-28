package main_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Each shell loads the snippet as the README says.

// shell runs a script in the named shell, or skips the test if the shell is not installed.
func (mach machine) shell(t *testing.T, name string, args ...string) string {
	t.Helper()

	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("%s is not installed", name)
	}

	if err := os.Symlink(bin, filepath.Join(mach.path, "claude-complete")); err != nil && !os.IsExist(err) {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = mach.dir
	cmd.Env = []string{
		"HOME=" + mach.home,
		"XDG_CACHE_HOME=" + mach.cache,
		"PATH=" + mach.path + ":" + os.Getenv("PATH"),
		"PWD=" + mach.dir,
	}

	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("%s gave no answer within the deadline, what it printed:\n%s", name, out)
	}

	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}

	return string(out)
}

// bashScript sets the line, the cursor and the words, as readline does, and calls the completion function.
const bashScript = `
[ "${BASH_VERSINFO[0]}${BASH_VERSINFO[1]}" -ge 44 ] || { echo "SKIP bash $BASH_VERSION"; exit 0; }
source <(claude-complete _carapace bash)
comp() {
  COMP_LINE="$1"; COMP_POINT=${#COMP_LINE}
  read -ra COMP_WORDS <<<"$COMP_LINE"; [[ "$COMP_LINE" == *" " ]] && COMP_WORDS+=("")
  COMP_CWORD=$(( ${#COMP_WORDS[@]} - 1 )); COMPREPLY=()
  _claude_completion claude "${COMP_WORDS[COMP_CWORD]}" "${COMP_WORDS[COMP_CWORD-1]}"
  printf '%s|' "${COMPREPLY[@]}"; echo
}
comp 'claude --permission-mode pl'
comp 'claude --permission-mode=pl'
comp 'claude -r ta'
`

func TestBash(t *testing.T) {
	t.Parallel()

	out := newMachine(t).shell(t, "bash", "-c", bashScript)
	if strings.HasPrefix(out, "SKIP") {
		t.Skip(out)
	}

	// readline replaces only what follows "=", which is in COMP_WORDBREAKS
	if want := strings.Join([]string{"plan|", "plan|", "talos-client-cache|", ""}, "\n"); out != want {
		t.Errorf("got\n%s\nwant\n%s", out, want)
	}
}

func TestFish(t *testing.T) {
	t.Parallel()

	out := newMachine(t).shell(t, "fish", "--no-config", "-c", `claude-complete _carapace fish | source
complete -C "claude --permission-mode pl"
complete -C "claude -r ta"`)

	for _, want := range []string{"plan", "talos-client-cache"} {
		if !strings.Contains(out, want) {
			t.Errorf("got\n%s\nwant %q in it", out, want)
		}
	}
}

// zshScript types a line and a tab in a pseudo terminal. Every step waits for the next prompt, and the
// completion for its text.
const zshScript = `
zmodload zsh/zpty
zpty -b z 'PS1="READY> " zsh -f'
zpty -r z out '*READY> *'
step() { zpty -w z "$1" && zpty -r z out '*READY> *' }
step 'autoload -Uz compinit && compinit -u'
step 'source <(claude-complete _carapace zsh)'
zpty -w -n z 'claude --resume talos'$'\t'
zpty -r z out '*-client-cache*' && print -r -- "completed"
zpty -d z
`

func TestZsh(t *testing.T) {
	t.Parallel()

	if out := newMachine(t).shell(t, "zsh", "-f", "-c", zshScript); !strings.Contains(out, "completed") {
		t.Errorf("the line was not completed:\n%q", out)
	}
}
