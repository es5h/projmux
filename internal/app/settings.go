package app

import (
	"errors"
	"fmt"
	"io"
	"strings"

	intfzf "github.com/es5h/projmux/internal/ui/fzf"
)

type settingsCommand struct {
	ai       *aiCommand
	switcher *switchCommand
	runner   intfzf.Runner
}

func newSettingsCommand(ai *aiCommand, switcher *switchCommand) *settingsCommand {
	return &settingsCommand{
		ai:       ai,
		switcher: switcher,
		runner:   intfzf.NewRunner(),
	}
}

func (c *settingsCommand) Run(args []string, stdout, stderr io.Writer) error {
	if len(args) != 0 {
		printSettingsUsage(stderr)
		return errors.New("settings does not accept positional arguments")
	}
	if c.runner == nil {
		return errors.New("settings runner is not configured")
	}

	for {
		entries, err := c.entries()
		if err != nil {
			return err
		}

		result, err := c.runner.Run(intfzf.Options{
			UI:         "settings",
			Entries:    entries,
			Prompt:     "Settings > ",
			Header:     "AI defaults and project picker pins",
			Footer:     projmuxFooter("Enter: apply  |  Esc/Alt+5/Ctrl+Alt+S: close"),
			ExpectKeys: []string{"enter"},
			Bindings: []string{
				"esc:abort",
				"ctrl-c:abort",
				"alt-5:abort",
				"ctrl-alt-s:abort",
			},
		})
		if err != nil {
			if isNoSelectionExit(err) {
				return nil
			}
			return fmt.Errorf("run settings picker: %w", err)
		}
		if result.Key != "enter" || strings.TrimSpace(result.Value) == "" {
			return nil
		}
		if err := c.execute(result.Value, stdout, stderr); err != nil {
			return err
		}
	}
}

func (c *settingsCommand) entries() ([]intfzf.Entry, error) {
	entries := c.aiEntries()

	if c.switcher != nil {
		switchEntries, err := c.switcher.settingsEntries()
		if err != nil {
			return nil, err
		}
		for _, entry := range switchEntries {
			entries = append(entries, intfzf.Entry{
				Label: "\x1b[36mProject Picker\x1b[0m  " + entry.Label,
				Value: "switch:" + entry.Value,
			})
		}
	}
	return entries, nil
}

func (c *settingsCommand) aiEntries() []intfzf.Entry {
	if c.ai == nil {
		return nil
	}

	rows := c.ai.settingsRows()
	entries := make([]intfzf.Entry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, intfzf.Entry{
			Label: "\x1b[35mAI\x1b[0m              " + row.Label,
			Value: "ai:" + row.Value,
		})
	}
	return entries
}

func (c *settingsCommand) execute(value string, stdout, stderr io.Writer) error {
	switch {
	case strings.HasPrefix(value, "ai:"):
		mode := strings.TrimPrefix(value, "ai:")
		if c.ai == nil {
			return errors.New("ai settings are not configured")
		}
		return c.ai.setMode(mode)
	case strings.HasPrefix(value, "switch:"):
		action := strings.TrimPrefix(value, "switch:")
		if c.switcher == nil {
			return errors.New("project picker settings are not configured")
		}
		return c.switcher.executeSettingsAction(action, stdout, stderr)
	default:
		printSettingsUsage(stderr)
		return fmt.Errorf("unknown settings action: %s", value)
	}
}

func printSettingsUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  projmux settings")
}
