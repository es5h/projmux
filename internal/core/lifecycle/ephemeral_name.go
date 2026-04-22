package lifecycle

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var ephemeralNameSanitizer = regexp.MustCompile(`[^A-Za-z0-9_-]`)

// EphemeralSessionName returns the legacy shell-compatible ephemeral session
// name derived from the current working directory basename plus a timestamp.
func EphemeralSessionName(cwd string, now time.Time) string {
	base := filepath.Base(filepath.Clean(strings.TrimSpace(cwd)))
	if base == "." || base == string(filepath.Separator) || base == "" {
		base = "shell"
	}

	base = ephemeralNameSanitizer.ReplaceAllString(base, "-")
	if base == "" {
		base = "shell"
	}

	return base + "-" + now.Format("20060102-150405")
}
