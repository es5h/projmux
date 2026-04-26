package sessions

import (
	"path/filepath"
	"strings"
)

// Namer resolves directory paths to stable tmux session names.
type Namer struct {
	homeDir string
}

// NewNamer builds a session namer for the given home directory.
func NewNamer(homeDir string) Namer {
	cleanHome := filepath.Clean(homeDir)

	return Namer{
		homeDir: cleanHome,
	}
}

// SessionName returns the tmux session name for the provided directory.
func (n Namer) SessionName(dir string) string {
	switch dir {
	case n.homeDir:
		return "home"
	}

	trimmedDir := trimTrailingSeparators(dir)
	base := filepath.Base(trimmedDir)
	parent := filepath.Base(filepath.Dir(trimmedDir))

	if parent == "." || parent == string(filepath.Separator) || parent == "" {
		return Sanitize(base)
	}

	return Sanitize(parent + "-" + base)
}

// Sanitize applies the legacy shell replacements used for tmux session names.
func Sanitize(name string) string {
	replacer := strings.NewReplacer(
		".", "_",
		":", "_",
		"/", "-",
		" ", "-",
	)

	return replacer.Replace(name)
}

func trimTrailingSeparators(path string) string {
	if path == "" {
		return ""
	}

	trimmed := strings.TrimRight(path, string(filepath.Separator))
	if trimmed == "" {
		return string(filepath.Separator)
	}

	return trimmed
}
