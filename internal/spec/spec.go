// Package spec reads the command tree of the installed claude CLI from its help output.
package spec

import "slices"

// Spec represents the command tree of one claude version.
type Spec struct {
	Version string   `json:"version"`
	Binary  string   `json:"binary,omitempty"` // see Identify
	Root    *Command `json:"root"`
}

// Command represents one node of the command tree.
type Command struct {
	Name     string     `json:"name,omitempty"`
	Aliases  []string   `json:"aliases,omitempty"`
	Desc     string     `json:"desc,omitempty"`
	Flags    []Flag     `json:"flags,omitempty"`
	Commands []*Command `json:"commands,omitempty"`
}

// Flag represents one option of a command.
type Flag struct {
	Names   []string `json:"names"`         // short first, e.g., "-r", "--resume"
	Arg     string   `json:"arg,omitempty"` // e.g., "<port>", "[value]", "<dirs...>", empty for a boolean
	Desc    string   `json:"desc,omitempty"`
	Choices []string `json:"choices,omitempty"`
}

// Long returns the long name of the flag.
func (f Flag) Long() string {
	if len(f.Names) == 0 {
		return ""
	}

	return f.Names[len(f.Names)-1]
}

// Optional reports whether the flag's value may be omitted, as in "--resume [value]".
func (f Flag) Optional() bool {
	return len(f.Arg) > 0 && f.Arg[0] == '['
}

// Variadic reports whether the flag takes more than one value, as in "--add-dir <directories...>".
func (f Flag) Variadic() bool {
	return len(f.Arg) > 4 && f.Arg[len(f.Arg)-4:len(f.Arg)-1] == "..."
}

// Find returns the subcommand called name, or nil.
func (c *Command) Find(name string) *Command {
	for _, sub := range c.Commands {
		if sub.Name == name || slices.Contains(sub.Aliases, name) {
			return sub
		}
	}

	return nil
}

// valid reports whether the command and everything under it has the shape the parser produces.
func (c *Command) valid() bool {
	if c == nil {
		return false
	}

	for _, flag := range c.Flags {
		if len(flag.Names) == 0 {
			return false
		}
	}

	for _, sub := range c.Commands {
		if sub == nil || sub.Name == "" || !sub.valid() {
			return false
		}
	}

	return true
}
