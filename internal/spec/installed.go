package spec

import (
	"path/filepath"
	"regexp"
)

var (
	// the native installer, e.g., ~/.local/share/claude/versions/2.1.283.
	nativeRe = regexp.MustCompile(`/claude/versions/(\d+\.\d+\.\d+)$`)
	// the Homebrew casks, e.g., /opt/homebrew/Caskroom/claude-code/2.1.283/claude.
	caskRe = regexp.MustCompile(`/Caskroom/claude-code(?:@latest)?/(\d+\.\d+\.\d+)/claude$`)
)

// VersionFromPath returns the version of the claude binary at path, read from where it is installed, or ""
// when the layout is not known, e.g., for a wrapper script.
func VersionFromPath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return ""
	}

	for _, re := range []*regexp.Regexp{nativeRe, caskRe} {
		if match := re.FindStringSubmatch(filepath.ToSlash(resolved)); match != nil {
			return match[1]
		}
	}

	return ""
}
