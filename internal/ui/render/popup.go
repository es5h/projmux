package render

import (
	"strconv"
	"strings"

	"github.com/es5h/projmux/internal/core/preview"
)

// RenderPopupPreview renders a concise textual popup preview from the derived
// preview read-model.
func RenderPopupPreview(model preview.PopupReadModel) string {
	var builder strings.Builder

	builder.WriteString("session: ")
	builder.WriteString(sanitizeCell(model.SessionName))
	builder.WriteString("\n")

	builder.WriteString("summary: ")
	builder.WriteString(formatPopupSummary(model))
	builder.WriteString("\n")

	builder.WriteString("selected: ")
	builder.WriteString(formatSelectedSummary(model))
	builder.WriteString("\n")

	builder.WriteString("windows:\n")
	writeWindows(&builder, model)

	builder.WriteString("panes:\n")
	writePanes(&builder, model)

	return builder.String()
}

func formatPopupSummary(model preview.PopupReadModel) string {
	var parts []string
	windowCount := model.WindowCount
	if windowCount == 0 {
		windowCount = len(model.Windows)
	}
	totalPaneCount := model.TotalPaneCount
	if totalPaneCount == 0 {
		totalPaneCount = len(model.Panes)
	}
	parts = append(parts, sanitizeCell(strconv.Itoa(windowCount))+"w")
	parts = append(parts, sanitizeCell(strconv.Itoa(totalPaneCount))+"p")
	if target := formatTargetSummary(model.SelectedWindowIndex, model.SelectedPaneIndex); target != "" {
		parts = append(parts, target)
	}
	return strings.Join(parts, "  ")
}

func formatSelectedSummary(model preview.PopupReadModel) string {
	if !model.HasSelection {
		return "none"
	}

	return formatTargetSummary(model.SelectedWindowIndex, model.SelectedPaneIndex)
}

func formatTargetSummary(windowIndex, paneIndex string) string {
	windowIndex = strings.TrimSpace(windowIndex)
	paneIndex = strings.TrimSpace(paneIndex)
	if windowIndex == "" {
		return ""
	}
	if paneIndex == "" {
		return "w" + sanitizeCell(windowIndex)
	}
	return "w" + sanitizeCell(windowIndex) + ".p" + sanitizeCell(paneIndex)
}

func writeWindows(builder *strings.Builder, model preview.PopupReadModel) {
	if len(model.Windows) == 0 {
		builder.WriteString("  (none)\n")
		return
	}

	selectedWindow := strings.TrimSpace(model.SelectedWindowIndex)
	for _, window := range model.Windows {
		builder.WriteString("  ")
		builder.WriteString(selectionMarker(window.Index == selectedWindow))
		builder.WriteString(" ")
		builder.WriteString(formatWindowSummary(window))
		builder.WriteString("\n")
	}
}

func writePanes(builder *strings.Builder, model preview.PopupReadModel) {
	if len(model.Panes) == 0 {
		builder.WriteString("  (none)\n")
		return
	}

	selectedPane := strings.TrimSpace(model.SelectedPaneIndex)
	for _, pane := range model.Panes {
		builder.WriteString("  ")
		builder.WriteString(selectionMarker(pane.Index == selectedPane && selectedPane != ""))
		builder.WriteString(" ")
		builder.WriteString(formatPaneSummary(pane))
		builder.WriteString("\n")
	}
}

func selectionMarker(selected bool) string {
	if selected {
		return "*"
	}
	return " "
}

func formatWindowSummary(window preview.Window) string {
	parts := []string{sanitizeCell(window.Index)}

	name := sanitizeCell(window.Name)
	if name != "" {
		parts = append(parts, name)
	}
	if window.PaneCount > 0 {
		parts = append(parts, sanitizeCell(strconv.Itoa(window.PaneCount))+" panes")
	}
	path := sanitizeCell(window.Path)
	if path != "" {
		parts = append(parts, path)
	}

	return strings.Join(parts, " | ")
}

func formatPaneSummary(pane preview.Pane) string {
	parts := []string{sanitizeCell(pane.Index)}

	title := sanitizeCell(pane.Title)
	if title != "" {
		parts = append(parts, title)
	}
	command := sanitizeCell(pane.Command)
	if command != "" {
		parts = append(parts, command)
	}
	path := sanitizeCell(pane.Path)
	if path != "" {
		parts = append(parts, path)
	}

	return strings.Join(parts, " | ")
}
