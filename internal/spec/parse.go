package spec

import (
	"regexp"
	"strings"
)

// page represents the parsed content of one help output.
type page struct {
	usage    string // e.g., "claude mcp add [options] <name>"
	desc     string
	flags    []Flag
	commands []*Command
}

var (
	headerRe  = regexp.MustCompile(`^[A-Z][A-Za-z ]*:$`)
	gapRe     = regexp.MustCompile(`\s{2,}`)
	commandRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	flagRe    = regexp.MustCompile(`^(-[A-Za-z0-9?]|--[A-Za-z0-9][A-Za-z0-9-]*)$`)

	choicesRe     = regexp.MustCompile(`\(choices:([^)]*)\)`) // the values end at the first `key:`
	choiceTokenRe = regexp.MustCompile(`"([^"]*)"|([A-Za-z-]+):`)

	proseChoicesRe = regexp.MustCompile(`\(((?:[a-z][\w-]*, )+(?:or )?[a-z][\w-]*)\)`) // e.g., "(low, high, max)"
)

// row represents one entry of an Options or Commands section.
type row struct {
	first string
	rest  []string
}

// parser reads a help output line by line.
type parser struct {
	page    page
	section string
	current *row
	desc    []string
}

// parse reads one commander help output.
//
// A row starts at exactly two spaces of indent, and every deeper-indented line belongs to the row above.
func parse(help string) page {
	var state parser

	for line := range strings.SplitSeq(help, "\n") {
		state.line(strings.TrimRight(line, " \r"))
	}

	state.flush()

	state.page.desc = strings.Join(state.desc, " ")

	return state.page
}

func (p *parser) line(line string) {
	switch {
	case p.page.usage == "" && strings.HasPrefix(line, "Usage: "):
		p.page.usage = strings.TrimPrefix(line, "Usage: ")
		p.section = "description"

		return
	case headerRe.MatchString(line):
		p.flush()
		p.section = strings.ToLower(strings.TrimSuffix(line, ":"))

		return
	}

	switch p.section {
	case "description":
		if line != "" {
			p.desc = append(p.desc, line)
		}
	case "options", "commands":
		p.rowLine(line)
	}
}

// rowLine reads a line of an Options or Commands section.
func (p *parser) rowLine(line string) {
	switch {
	case strings.HasPrefix(line, "   "):
		if p.current != nil {
			p.current.rest = append(p.current.rest, strings.TrimSpace(line))
		}
	case strings.HasPrefix(line, "  "):
		p.flush()
		p.current = &row{first: line[2:]}
	default:
		p.flush()
	}
}

func (p *parser) flush() {
	if p.current == nil {
		return
	}

	switch p.section {
	case "options":
		if flag, ok := parseFlag(*p.current); ok {
			p.page.flags = append(p.page.flags, flag)
		}
	case "commands":
		if cmd, ok := parseCommand(*p.current); ok {
			p.page.commands = append(p.page.commands, cmd)
		}
	}

	p.current = nil
}

// split separates a row into its name and its description, on the first line only: a name too long for
// the column stands alone on it.
func (r row) split() (string, string) {
	spec, desc := r.first, ""

	if loc := gapRe.FindStringIndex(r.first); loc != nil {
		spec, desc = r.first[:loc[0]], r.first[loc[1]:]
	}

	return strings.TrimSpace(spec), strings.Join(strings.Fields(desc+" "+strings.Join(r.rest, " ")), " ")
}

func parseFlag(r row) (Flag, bool) {
	spec, desc := r.split()

	var flag Flag

	if i := strings.IndexAny(spec, "<["); i >= 0 {
		flag.Arg = strings.TrimSpace(spec[i:])
		spec = spec[:i]
	}

	for name := range strings.SplitSeq(spec, ",") {
		name = strings.TrimSpace(name)
		if !flagRe.MatchString(name) {
			return Flag{}, false
		}

		flag.Names = append(flag.Names, name)
	}

	if len(flag.Names) == 0 {
		return Flag{}, false
	}

	flag.Desc = desc
	if flag.Arg != "" {
		flag.Choices = choices(desc)
	}

	return flag, true
}

func parseCommand(r row) (*Command, bool) {
	spec, desc := r.split()

	names, _, _ := strings.Cut(spec, " ")

	aliases := strings.Split(names, "|")
	for _, name := range aliases {
		if !commandRe.MatchString(name) {
			return nil, false
		}
	}

	if aliases[0] == "help" { // commander's own help command
		return nil, false
	}

	return &Command{Name: aliases[0], Aliases: aliases[1:], Desc: desc}, true
}

func choices(desc string) []string {
	if match := choicesRe.FindStringSubmatch(desc); match != nil {
		var values []string

		for _, token := range choiceTokenRe.FindAllStringSubmatch(match[1], -1) {
			if token[2] != "" {
				break
			}

			if token[1] != "" {
				values = append(values, token[1])
			}
		}

		return values
	}

	if match := proseChoicesRe.FindStringSubmatch(desc); match != nil {
		values := strings.Split(match[1], ", ")
		for i, value := range values {
			values[i] = strings.TrimPrefix(value, "or ")
		}

		return values
	}

	return nil
}
