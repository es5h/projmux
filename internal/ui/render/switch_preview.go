package render

import (
	"strings"

	corepreview "github.com/es5h/projmux/internal/core/preview"
)

// RenderSwitchPreview renders sidebar/popup preview context for a switch
// candidate.
func RenderSwitchPreview(model corepreview.SwitchReadModel) string {
	var builder strings.Builder

	builder.WriteString("dir: ")
	builder.WriteString(sanitizeSwitchPreviewPath(model))
	builder.WriteString("\n")

	builder.WriteString("session: ")
	builder.WriteString(sanitizeCell(model.SessionName))
	builder.WriteString("\n")

	builder.WriteString("state: ")
	builder.WriteString(sanitizeCell(model.SessionMode))
	builder.WriteString("\n")

	builder.WriteString("summary: ")
	builder.WriteString(formatPopupSummary(model.Popup))
	builder.WriteString("\n")

	builder.WriteString("selected: ")
	builder.WriteString(formatSelectedSummary(model.Popup))
	builder.WriteString("\n")

	builder.WriteString("windows:\n")
	writeWindows(&builder, model.Popup)

	builder.WriteString("panes:\n")
	writePanes(&builder, model.Popup)

	return builder.String()
}

func sanitizeSwitchPreviewPath(model corepreview.SwitchReadModel) string {
	path := sanitizeCell(model.DisplayPath)
	if path != "" {
		return path
	}

	path = sanitizeCell(model.Path)
	if path != "" {
		return path
	}

	return "-"
}
