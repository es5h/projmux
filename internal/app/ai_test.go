package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	intfzf "github.com/es5h/projmux/internal/ui/fzf"
)

func TestAISettingsGetAndSetMode(t *testing.T) {
	home := t.TempDir()
	cmd := testAICommand(home)

	var stdout bytes.Buffer
	if err := cmd.Run([]string{"settings", "--get"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run settings --get error = %v", err)
	}
	if got, want := stdout.String(), "selective\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}

	if err := cmd.Run([]string{"settings", "--set", "codex"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run settings --set error = %v", err)
	}
	stdout.Reset()
	if err := cmd.Run([]string{"settings", "--get"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run settings --get after set error = %v", err)
	}
	if got, want := stdout.String(), "codex\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestAISettingsPickerSetsSelectedMode(t *testing.T) {
	home := t.TempDir()
	runner := &capturingAIRunner{result: intfzf.Result{Key: "enter", Value: "shell"}}
	cmd := testAICommand(home)
	cmd.runner = runner

	if err := cmd.Run([]string{"settings"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run settings picker error = %v", err)
	}
	if got, want := runner.options.UI, "ai-settings"; got != want {
		t.Fatalf("runner UI = %q, want %q", got, want)
	}
	if got, want := runner.options.Prompt, "AI Setting > "; got != want {
		t.Fatalf("runner prompt = %q, want %q", got, want)
	}
	if got, want := readModeFile(t, home), "shell\n"; got != want {
		t.Fatalf("mode file = %q, want %q", got, want)
	}
}

func TestAISplitSelectiveOpensPickerPopup(t *testing.T) {
	home := t.TempDir()
	cmd := testAICommand(home)
	cmd.executable = func() (string, error) { return "/tmp/projmux bin", nil }
	cmd.lookupEnv = func(name string) string {
		if name == "TMUX" {
			return "/tmp/tmux"
		}
		return ""
	}
	cmd.readCommand = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name == "tmux" && reflect.DeepEqual(args, []string{"display-message", "-p", "-F", "#{pane_id}"}) {
			return []byte("%7\n"), nil
		}
		if name == "tmux" && reflect.DeepEqual(args, []string{"display-message", "-p", "-F", "#{client_width}"}) {
			return []byte("200\n"), nil
		}
		if name == "tmux" && reflect.DeepEqual(args, []string{"display-message", "-p", "-F", "#{client_height}"}) {
			return []byte("50\n"), nil
		}
		return nil, os.ErrNotExist
	}

	if err := cmd.Run([]string{"split", "right"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run split error = %v", err)
	}

	want := []recordedAICommand{{
		name: "tmux",
		args: []string{"display-popup", "-E", "-w", "80", "-h", "15", "TMUX_SPLIT_TARGET_PANE='%7' '/tmp/projmux bin' ai picker --inside 'right'"},
	}}
	if !reflect.DeepEqual(cmdRecorder(cmd).commands, want) {
		t.Fatalf("commands = %#v, want %#v", cmdRecorder(cmd).commands, want)
	}
}

func TestAISplitShellUsesTmuxSplitWindow(t *testing.T) {
	home := t.TempDir()
	work := filepath.Join(home, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := testAICommand(home)
	if err := cmd.setMode("shell"); err != nil {
		t.Fatal(err)
	}
	cmd.lookupEnv = func(name string) string {
		switch name {
		case "TMUX":
			return "/tmp/tmux"
		case "TMUX_SPLIT_CONTEXT_DIR":
			return work
		case "TMUX_SPLIT_TARGET_PANE":
			return "%9"
		default:
			return ""
		}
	}
	cmd.readCommand = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name == "tmux" && reflect.DeepEqual(args, []string{"display-message", "-p", "-t", "%9", "-F", "#{pane_id}"}) {
			return []byte("%9\n"), nil
		}
		return nil, os.ErrNotExist
	}

	if err := cmd.Run([]string{"split", "down"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run split shell error = %v", err)
	}

	want := []recordedAICommand{
		{name: "tmux", args: []string{"display-message", "ai split default: shell"}},
		{name: "tmux", args: []string{"split-window", "-v", "-t", "%9", "-c", work, "zsh", "-l"}},
	}
	if !reflect.DeepEqual(cmdRecorder(cmd).commands, want) {
		t.Fatalf("commands = %#v, want %#v", cmdRecorder(cmd).commands, want)
	}
}

type capturingAIRunner struct {
	options intfzf.Options
	result  intfzf.Result
	err     error
}

func (r *capturingAIRunner) Run(options intfzf.Options) (intfzf.Result, error) {
	r.options = options
	return r.result, r.err
}

type recordedAICommand struct {
	name string
	args []string
}

type aiCommandRecorder struct {
	commands []recordedAICommand
}

func testAICommand(home string) *aiCommand {
	recorder := &aiCommandRecorder{}
	cmd := &aiCommand{
		runner:     &capturingAIRunner{},
		executable: func() (string, error) { return "/tmp/projmux", nil },
		lookupEnv: func(name string) string {
			switch name {
			case "HOME":
				return home
			default:
				return ""
			}
		},
		homeDir: func() (string, error) { return home, nil },
		runCommand: func(_ context.Context, name string, args ...string) error {
			recorder.commands = append(recorder.commands, recordedAICommand{name: name, args: append([]string(nil), args...)})
			return nil
		},
		readCommand: func(context.Context, string, ...string) ([]byte, error) {
			return nil, os.ErrNotExist
		},
	}
	cmd.now = func() time.Time { return time.Unix(0, 0) }
	aiRecorders[cmd] = recorder
	return cmd
}

var aiRecorders = map[*aiCommand]*aiCommandRecorder{}

func cmdRecorder(cmd *aiCommand) *aiCommandRecorder {
	return aiRecorders[cmd]
}

func readModeFile(t *testing.T, home string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(home, ".config", "dotfiles", "tmux-ai-split-mode"))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
