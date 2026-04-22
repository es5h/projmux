package render

import (
	"path/filepath"
	"strings"
)

type SwitchRow struct {
	Label string
	Value string
}

type SwitchCandidate struct {
	Path        string
	DisplayPath string
	SessionName string
	ModeLabel   string
}

func PrettyPath(path, homeDir, repoRoot string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}

	path = filepath.Clean(path)
	homeDir = cleanPrettyRoot(homeDir)
	repoRoot = cleanPrettyRoot(repoRoot)

	if repoRoot != "" {
		if path == repoRoot {
			return "~rp"
		}
		if strings.HasPrefix(path, repoRoot+string(filepath.Separator)) {
			return "~rp" + strings.TrimPrefix(path, repoRoot)
		}
	}

	if homeDir != "" {
		if path == homeDir {
			return "~"
		}
		if strings.HasPrefix(path, homeDir+string(filepath.Separator)) {
			return "~" + strings.TrimPrefix(path, homeDir)
		}
	}

	return path
}

func BuildSwitchRows(candidates []SwitchCandidate) []SwitchRow {
	rows := make([]SwitchRow, 0, len(candidates))
	for _, candidate := range candidates {
		rows = append(rows, SwitchRow{
			Label: formatSwitchLabel(candidate),
			Value: candidate.Path,
		})
	}
	return rows
}

func formatSwitchLabel(candidate SwitchCandidate) string {
	parts := make([]string, 0, 3)

	sessionName := sanitizeCell(candidate.SessionName)
	if sessionName != "" {
		parts = append(parts, sessionName)
	}

	modeLabel := sanitizeCell(candidate.ModeLabel)
	if modeLabel != "" {
		parts = append(parts, "["+modeLabel+"]")
	}

	path := sanitizeCell(candidate.DisplayPath)
	if path == "" {
		path = sanitizeCell(candidate.Path)
	}
	if path != "" {
		parts = append(parts, path)
	}

	return strings.Join(parts, "  ")
}

func sanitizeCell(value string) string {
	value = strings.ReplaceAll(value, "\t", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.TrimSpace(value)
}

func cleanPrettyRoot(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}

	return filepath.Clean(path)
}
