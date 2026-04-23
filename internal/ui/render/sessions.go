package render

import (
	"strconv"
	"strings"

	inttmux "github.com/es5h/projmux/internal/integrations/tmux"
)

type SessionRow struct {
	Label string
	Value string
}

func BuildSessionRows(summaries []inttmux.RecentSessionSummary) []SessionRow {
	rows := make([]SessionRow, 0, len(summaries))
	for _, summary := range summaries {
		label := sanitizeCell(summary.Name)
		status := "detached"
		if summary.Attached {
			status = "attached"
		}
		label += "  [" + status + "]"
		if summary.WindowCount > 0 {
			label += "  " + sanitizeCell(strconv.Itoa(summary.WindowCount)) + "w"
		}
		if summary.PaneCount > 0 {
			label += "  " + sanitizeCell(strconv.Itoa(summary.PaneCount)) + "p"
		}
		if path := sanitizeCell(strings.TrimSpace(summary.Path)); path != "" {
			label += "  " + path
		}

		rows = append(rows, SessionRow{
			Label: label,
			Value: sanitizeCell(summary.Name),
		})
	}
	return rows
}
