package tree

import (
	"slices"
	"strings"

	"github.com/utkuozdemir/claude-complete/internal/spec"
)

// scan represents a command line as commander reads it.
type scan struct {
	words []string   // the typed words, rewritten for pflag
	path  []string   // the subcommand reached
	slot  *spec.Flag // the flag the word under the cursor is a value of, if pflag cannot tell
}

// scanner reads a command line word by word.
type scanner struct {
	scan

	node       *spec.Command
	positional bool       // no subcommand can follow
	variadic   *spec.Flag // being read
}

// normalize reads the typed words, all but the one under the cursor, the way commander does.
//
// pflag has no optional values (`--resume [value]`) and no variadic ones (`--add-dir <dirs...>`), so every
// value is written as `--flag=value`: `--resume mcp` becomes `--resume=mcp`, and `--add-dir a b` becomes
// `--add-dir=a --add-dir=b`.
func normalize(root *spec.Command, line []string) scan {
	if len(line) == 0 {
		return scan{}
	}

	state := scanner{node: root}
	typed := line[:len(line)-1]

	for idx := 0; idx < len(typed); idx++ {
		word := typed[idx]

		switch {
		case word == "--":
			state.words = append(state.words, typed[idx:]...)

			return state.scan
		case state.variadic != nil && !strings.HasPrefix(word, "-"):
			state.words = append(state.words, state.variadic.Long()+"="+word)
		case strings.HasPrefix(word, "-") && len(word) > 1:
			idx += state.flag(typed, idx)
		default:
			state.operand(word)
		}
	}

	state.slot = state.cursorFlag(typed)

	return state.scan
}

// cursorFlag returns the variadic flag being read, or else a bare optional flag right before the cursor.
func (s *scanner) cursorFlag(typed []string) *spec.Flag {
	if s.variadic != nil {
		return s.variadic
	}

	if len(typed) == 0 {
		return nil
	}

	if flag, ok := lookup(s.node, typed[len(typed)-1]); ok && flag.Optional() {
		return &flag
	}

	return nil
}

// flag reads the flag at idx and returns how many following words it took as its value.
func (s *scanner) flag(typed []string, idx int) int {
	s.variadic = nil

	name, value, hasValue := strings.Cut(typed[idx], "=")

	flag, ok := lookup(s.node, name)
	if !ok {
		// unknown, or a series of short flags
		s.words = append(s.words, typed[idx])

		return 0
	}

	if hasValue {
		s.words = append(s.words, flag.Long()+"="+value)

		return 0
	}

	next := idx + 1
	takes := flag.Arg != "" && next < len(typed) && (!flag.Optional() || !strings.HasPrefix(typed[next], "-"))

	if !takes {
		s.words = append(s.words, flag.Long())

		return 0
	}

	s.words = append(s.words, flag.Long()+"="+typed[next])

	if flag.Variadic() {
		s.variadic = &flag
	}

	return 1
}

// operand reads a word that is not a flag, a subcommand or a positional argument.
func (s *scanner) operand(word string) {
	s.words = append(s.words, word)

	if sub := s.node.Find(word); sub != nil && !s.positional {
		s.node = sub
		s.path = append(s.path, sub.Name)

		return
	}

	s.positional = true
}

// lookup returns the flag of cmd called name.
func lookup(cmd *spec.Command, name string) (spec.Flag, bool) {
	for _, flag := range cmd.Flags {
		if slices.Contains(flag.Names, name) {
			return flag, true
		}
	}

	return spec.Flag{}, false
}
