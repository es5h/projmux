package app

import (
	"bytes"
	"strings"
	"testing"

	"github.com/es5h/projmux/internal/core/candidates"
	intfzf "github.com/es5h/projmux/internal/ui/fzf"
)

func TestSettingsHubSetsAIDefaultMode(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	ai := testAICommand(home)
	switcher := testSettingsSwitchCommand(t, &stubSwitchPinStore{})
	var calls int
	var firstOptions intfzf.Options
	cmd := &settingsCommand{
		ai:       ai,
		switcher: switcher,
		runner: switchRunnerFunc(func(options intfzf.Options) (intfzf.Result, error) {
			calls++
			if calls == 1 {
				firstOptions = options
				return intfzf.Result{Key: "enter", Value: "ai:codex"}, nil
			}
			return intfzf.Result{}, nil
		}),
	}

	if err := cmd.Run(nil, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got, want := firstOptions.UI, "settings"; got != want {
		t.Fatalf("settings UI = %q, want %q", got, want)
	}
	if got, want := firstOptions.Prompt, "Settings > "; got != want {
		t.Fatalf("settings prompt = %q, want %q", got, want)
	}
	if got, want := firstOptions.Footer, "[projmux]\nEnter: apply  |  Esc/Alt+5/Ctrl+Alt+S: close"; got != want {
		t.Fatalf("settings footer = %q, want %q", got, want)
	}
	if !hasEntryPrefix(firstOptions.Entries, "\x1b[35mAI\x1b[0m") {
		t.Fatalf("settings entries = %#v, want AI entries", firstOptions.Entries)
	}
	if !hasEntryPrefix(firstOptions.Entries, "\x1b[36mProject Picker\x1b[0m") {
		t.Fatalf("settings entries = %#v, want project picker entries", firstOptions.Entries)
	}
	if got, want := readModeFile(t, home), "codex\n"; got != want {
		t.Fatalf("mode file = %q, want %q", got, want)
	}
}

func TestSettingsHubRunsProjectPickerActions(t *testing.T) {
	t.Parallel()

	store := &stubSwitchPinStore{}
	switcher := testSettingsSwitchCommand(t, store)
	var calls int
	cmd := &settingsCommand{
		ai:       testAICommand(t.TempDir()),
		switcher: switcher,
		runner: switchRunnerFunc(func(options intfzf.Options) (intfzf.Result, error) {
			calls++
			if got, want := options.UI, "settings"; got != want {
				t.Fatalf("settings UI = %q, want %q", got, want)
			}
			if calls == 1 {
				return intfzf.Result{Key: "enter", Value: "switch:add:/home/tester/source/repos/app"}, nil
			}
			return intfzf.Result{}, nil
		}),
	}

	var stdout bytes.Buffer
	if err := cmd.Run(nil, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got, want := store.addCalls, []string{"/home/tester/source/repos/app"}; !equalStrings(got, want) {
		t.Fatalf("add calls = %q, want %q", got, want)
	}
	if got, want := stdout.String(), "pinned: /home/tester/source/repos/app\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestSettingsHubRejectsArguments(t *testing.T) {
	t.Parallel()

	cmd := &settingsCommand{}
	var stderr bytes.Buffer
	err := cmd.Run([]string{"extra"}, &bytes.Buffer{}, &stderr)
	if err == nil {
		t.Fatal("Run() error = nil, want error")
	}
	if !strings.Contains(stderr.String(), "projmux settings") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func testSettingsSwitchCommand(t *testing.T, store *stubSwitchPinStore) *switchCommand {
	t.Helper()

	return &switchCommand{
		discover: func(candidates.Inputs) ([]string, error) {
			return []string{"/home/tester/source/repos/app"}, nil
		},
		pinStore: func() (switchPinStore, error) { return store, nil },
		runner: switchRunnerFunc(func(intfzf.Options) (intfzf.Result, error) {
			return intfzf.Result{}, nil
		}),
		sessions:   &capturingSwitchSessionExecutor{},
		identity:   stubSwitchIdentityResolver{name: "app"},
		validate:   func(string) error { return nil },
		homeDir:    func() (string, error) { return "/home/tester", nil },
		workingDir: func() (string, error) { return "/home/tester/source/repos/app", nil },
		lookupEnv: func(name string) string {
			if name == repoRootEnvVar {
				return "/home/tester/source/repos"
			}
			return ""
		},
	}
}

func hasEntryPrefix(entries []intfzf.Entry, prefix string) bool {
	for _, entry := range entries {
		if strings.HasPrefix(entry.Label, prefix) {
			return true
		}
	}
	return false
}
