package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	intfzf "github.com/es5h/projmux/internal/ui/fzf"
)

const (
	aiModeSelective = "selective"
	aiModeClaude    = "claude"
	aiModeCodex     = "codex"
	aiModeShell     = "shell"
)

type aiCommandRunner interface {
	Run(options intfzf.Options) (intfzf.Result, error)
}

type aiCommand struct {
	runner      aiCommandRunner
	executable  func() (string, error)
	lookupEnv   func(string) string
	homeDir     func() (string, error)
	runCommand  func(ctx context.Context, name string, args ...string) error
	readCommand func(ctx context.Context, name string, args ...string) ([]byte, error)
	now         func() time.Time
}

func newAICommand() *aiCommand {
	return &aiCommand{
		runner:      intfzf.NewRunner(),
		executable:  os.Executable,
		lookupEnv:   os.Getenv,
		homeDir:     os.UserHomeDir,
		runCommand:  runExternalCommand,
		readCommand: readExternalCommand,
		now:         time.Now,
	}
}

func (c *aiCommand) Run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printAIUsage(stderr)
		return errors.New("ai requires a subcommand")
	}

	switch args[0] {
	case "split":
		return c.runSplit(args[1:], stderr)
	case "picker":
		return c.runPicker(args[1:], stderr)
	case "settings":
		return c.runSettings(args[1:], stdout, stderr)
	case "help", "--help", "-h":
		printAIUsage(stdout)
		return nil
	default:
		printAIUsage(stderr)
		return fmt.Errorf("unknown ai subcommand: %s", args[0])
	}
}

func (c *aiCommand) runSplit(args []string, stderr io.Writer) error {
	direction, err := parseAISplitDirection(args, "ai split", stderr)
	if err != nil {
		return err
	}

	mode := c.getMode()
	switch mode {
	case aiModeClaude:
		return c.runAgentSplit(aiModeClaude, direction)
	case aiModeCodex:
		return c.runAgentSplit(aiModeCodex, direction)
	case aiModeShell:
		return c.runShellSplit(direction)
	case aiModeSelective:
		return c.openPicker(direction)
	default:
		return c.openPicker(direction)
	}
}

func (c *aiCommand) runPicker(args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("ai picker", flag.ContinueOnError)
	fs.SetOutput(stderr)
	inside := fs.Bool("inside", false, "run inside an already-open popup")
	shellOnly := fs.Bool("shell", false, "open a plain shell split")
	if err := fs.Parse(args); err != nil {
		return err
	}
	direction, err := parseAISplitDirection(fs.Args(), "ai picker", stderr)
	if err != nil {
		return err
	}
	if *shellOnly {
		return c.runShellSplit(direction)
	}
	if !*inside && c.env("TMUX") != "" {
		return c.openPicker(direction)
	}

	result, err := c.runAgentPicker(direction)
	if err != nil {
		return err
	}
	if result.Value == "" || result.Key != "enter" {
		return nil
	}

	switch normalizeAIMode(result.Value) {
	case aiModeClaude:
		return c.runAgentSplit(aiModeClaude, direction)
	case aiModeCodex:
		return c.runAgentSplit(aiModeCodex, direction)
	case aiModeShell:
		return c.runShellSplit(direction)
	default:
		return nil
	}
}

func (c *aiCommand) runSettings(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("ai settings", flag.ContinueOnError)
	fs.SetOutput(stderr)
	get := fs.Bool("get", false, "print the configured AI split mode")
	set := fs.String("set", "", "set the configured AI split mode")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		printAIUsage(stderr)
		return errors.New("ai settings does not accept positional arguments")
	}

	if *get {
		_, err := fmt.Fprintln(stdout, c.getMode())
		return err
	}
	if strings.TrimSpace(*set) != "" {
		return c.setMode(*set)
	}

	if c.runner == nil {
		return errors.New("ai settings runner is not configured")
	}
	result, err := c.runner.Run(intfzf.Options{
		UI:         "ai-settings",
		Entries:    c.settingsRows(),
		Prompt:     "AI Setting > ",
		Header:     "Set Ctrl+Shift+R/L default mode",
		Footer:     "Enter: set default  |  Esc/Alt+5/Ctrl+Alt+S: close",
		ExpectKeys: []string{"enter"},
		Bindings: []string{
			"esc:abort",
			"ctrl-c:abort",
			"alt-5:abort",
			"ctrl-alt-s:abort",
			"alt-4:abort",
		},
	})
	if err != nil {
		return fmt.Errorf("run ai settings picker: %w", err)
	}
	if result.Key != "enter" || result.Value == "" {
		return nil
	}
	return c.setMode(result.Value)
}

func (c *aiCommand) runAgentPicker(direction string) (intfzf.Result, error) {
	if c.runner == nil {
		return intfzf.Result{}, errors.New("ai picker runner is not configured")
	}
	return c.runner.Run(intfzf.Options{
		UI:         "ai-picker",
		Entries:    c.agentRows(),
		Prompt:     "AI Launch > ",
		Header:     "Split Direction: " + direction + "  |  Choose runtime",
		Footer:     "Enter: launch  |  Esc/Alt+4/Alt+5/Ctrl+Alt+S: close",
		ExpectKeys: []string{"enter"},
		Bindings: []string{
			"esc:abort",
			"ctrl-c:abort",
			"alt-4:abort",
			"alt-5:abort",
			"ctrl-alt-s:abort",
		},
	})
}

func (c *aiCommand) settingsRows() []intfzf.Entry {
	current := c.getMode()
	modes := []struct {
		mode string
		desc string
	}{
		{aiModeSelective, "show picker each time"},
		{aiModeClaude, "always run Claude split"},
		{aiModeCodex, "always run Codex split"},
		{aiModeShell, "always open zsh split"},
	}
	rows := make([]intfzf.Entry, 0, len(modes))
	for _, item := range modes {
		tag := ansiDim("[ ]")
		if item.mode == current {
			tag = "\x1b[32m[ACTIVE]\x1b[0m"
		}
		rows = append(rows, intfzf.Entry{
			Label: fmt.Sprintf("%s \x1b[36m%-9s\x1b[0m  \x1b[90m%s\x1b[0m", tag, item.mode, item.desc),
			Value: item.mode,
		})
	}
	return rows
}

func (c *aiCommand) agentRows() []intfzf.Entry {
	rows := []intfzf.Entry{
		c.agentRow(aiModeClaude, "Anthropic CLI split"),
		c.agentRow(aiModeCodex, "OpenAI Codex split"),
		{
			Label: fmt.Sprintf("%-8s \x1b[34m[READY]\x1b[0m Plain zsh split (\x1b[90mno agent\x1b[0m)", aiModeShell),
			Value: aiModeShell,
		},
	}
	return rows
}

func (c *aiCommand) agentRow(mode, desc string) intfzf.Entry {
	status := "\x1b[33m[MISSING]\x1b[0m"
	if c.agentAvailable(mode) {
		status = "\x1b[32m[READY]\x1b[0m"
	}
	return intfzf.Entry{
		Label: fmt.Sprintf("%-8s %s %s", mode, status, desc),
		Value: mode,
	}
}

func (c *aiCommand) getMode() string {
	content, err := os.ReadFile(c.configFile())
	if err != nil {
		return aiModeSelective
	}
	return normalizeAIMode(strings.TrimSpace(string(content)))
}

func (c *aiCommand) setMode(mode string) error {
	mode = normalizeAIMode(mode)
	path := c.configFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(mode+"\n"), 0o644); err != nil {
		return err
	}
	_ = c.displayMessage("ai split default: " + mode)
	return nil
}

func (c *aiCommand) configFile() string {
	configHome := strings.TrimSpace(c.env("XDG_CONFIG_HOME"))
	if configHome == "" {
		homeDir, err := c.home()
		if err != nil || strings.TrimSpace(homeDir) == "" {
			return filepath.Join(".config", "dotfiles", "tmux-ai-split-mode")
		}
		configHome = filepath.Join(homeDir, ".config")
	}
	return filepath.Join(configHome, "dotfiles", "tmux-ai-split-mode")
}

func (c *aiCommand) openPicker(direction string) error {
	binaryPath, err := c.binaryPath()
	if err != nil {
		return err
	}
	targetPane := c.resolveTargetPane()
	command := shellEnv("TMUX_SPLIT_TARGET_PANE", targetPane) + shellQuote(binaryPath) + " ai picker --inside " + shellQuote(direction)
	width, height := c.popupSize(40, 64, 30, 12)
	return c.run("tmux", "display-popup", "-E", "-w", width, "-h", height, command)
}

func (c *aiCommand) runAgentSplit(mode, direction string) error {
	if mode == aiModeShell {
		return c.runShellSplit(direction)
	}
	if !c.agentAvailable(mode) {
		_ = c.displayMessage("selected runner is not installed: " + mode)
		return fmt.Errorf("selected runner is not installed: %s", mode)
	}
	return c.run(c.agentScript(mode), direction)
}

func (c *aiCommand) runShellSplit(direction string) error {
	targetPane := c.resolveTargetPane()
	contextDir := c.resolveContextDir()
	splitFlag := "-h"
	if direction == "down" {
		splitFlag = "-v"
	}

	args := []string{"split-window", splitFlag}
	if targetPane != "" {
		args = append(args, "-t", targetPane)
	}
	if contextDir != "" {
		args = append(args, "-c", contextDir)
	}
	args = append(args, "zsh", "-l")
	return c.run("tmux", args...)
}

func (c *aiCommand) resolveContextDir() string {
	if dir := c.env("TMUX_SPLIT_CONTEXT_DIR"); isDir(dir) {
		return dir
	}
	if c.env("TMUX") != "" {
		if path := c.readTrimmed("tmux", "display-message", "-p", "-F", "#{pane_current_path}"); isDir(path) {
			return path
		}
	}
	if target := c.resolveRecentTmuxTarget(); target != "" {
		if path := c.readTrimmed("tmux", "display-message", "-p", "-t", target, "-F", "#{pane_current_path}"); isDir(path) {
			return path
		}
	}
	return c.resolveIDEContextDir()
}

func (c *aiCommand) resolveTargetPane() string {
	if pane := strings.TrimSpace(c.env("TMUX_SPLIT_TARGET_PANE")); pane != "" {
		if resolved := c.readTrimmed("tmux", "display-message", "-p", "-t", pane, "-F", "#{pane_id}"); resolved != "" {
			return resolved
		}
		return pane
	}
	if c.env("TMUX") != "" {
		return c.readTrimmed("tmux", "display-message", "-p", "-F", "#{pane_id}")
	}
	if target := c.resolveRecentTmuxTarget(); target != "" {
		return c.readTrimmed("tmux", "display-message", "-p", "-t", target, "-F", "#{pane_id}")
	}
	return ""
}

func (c *aiCommand) resolveRecentTmuxTarget() string {
	out, err := c.read("tmux", "list-clients", "-F", "#{client_activity}\t#{session_id}")
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	bestActivity := ""
	bestTarget := ""
	for _, line := range lines {
		activity, target, ok := strings.Cut(line, "\t")
		if !ok || strings.TrimSpace(target) == "" {
			continue
		}
		if bestActivity == "" || strings.TrimSpace(activity) > bestActivity {
			bestActivity = strings.TrimSpace(activity)
			bestTarget = strings.TrimSpace(target)
		}
	}
	return bestTarget
}

func (c *aiCommand) resolveIDEContextDir() string {
	homeDir, err := c.home()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		return ""
	}
	cacheDir := filepath.Join(homeDir, ".cache", "ide")
	for _, name := range []string{"vscode", "code", "intellij-idea", "pycharm", "goland", "webstorm", "clion", "rider", "idea", "android-studio"} {
		content, err := os.ReadFile(filepath.Join(cacheDir, name))
		if err == nil {
			if path := strings.TrimSpace(string(content)); isDir(path) {
				return path
			}
		}
	}
	return ""
}

func (c *aiCommand) popupSize(widthPercent, widthMin, heightPercent, heightMin int) (string, string) {
	return c.popupAxisSize("width", widthPercent, widthMin), c.popupAxisSize("height", heightPercent, heightMin)
}

func (c *aiCommand) popupAxisSize(axis string, percent, minimum int) string {
	format := "#{client_width}"
	if axis == "height" {
		format = "#{client_height}"
	}
	total := parsePositiveInt(c.readTrimmed("tmux", "display-message", "-p", "-F", format))
	if total <= 0 {
		return fmt.Sprintf("%d%%", percent)
	}
	value := total * percent / 100
	if value < minimum {
		value = minimum
	}
	if value > total {
		value = total
	}
	return fmt.Sprintf("%d", value)
}

func (c *aiCommand) agentAvailable(mode string) bool {
	if mode == aiModeShell {
		return true
	}
	if !isExecutable(c.agentScript(mode)) {
		return false
	}
	return c.findAgentBinary(mode) != ""
}

func (c *aiCommand) findAgentBinary(mode string) string {
	switch mode {
	case aiModeClaude:
		return firstExecutable(c.readTrimmed("command", "-v", "claude"), filepath.Join(c.homeOrEmpty(), ".local", "bin", "claude"))
	case aiModeCodex:
		if path := firstExecutable(c.readTrimmed("command", "-v", "codex"), filepath.Join(c.homeOrEmpty(), ".npm-global", "bin", "codex"), filepath.Join(c.homeOrEmpty(), ".local", "bin", "codex")); path != "" {
			return path
		}
		matches, _ := filepath.Glob(filepath.Join(c.homeOrEmpty(), ".vscode", "extensions", "openai.chatgpt-*", "bin", "*", "codex"))
		return newestExecutable(matches)
	default:
		return ""
	}
}

func (c *aiCommand) agentScript(mode string) string {
	switch mode {
	case aiModeClaude:
		return filepath.Join(c.homeOrEmpty(), ".local", "bin", "tmux-claude-split.sh")
	case aiModeCodex:
		return filepath.Join(c.homeOrEmpty(), ".local", "bin", "tmux-codex-split.sh")
	default:
		return ""
	}
}

func (c *aiCommand) displayMessage(message string) error {
	if strings.TrimSpace(message) == "" {
		return nil
	}
	return c.run("tmux", "display-message", message)
}

func (c *aiCommand) binaryPath() (string, error) {
	if c.executable == nil {
		return "", errors.New("ai executable resolver is not configured")
	}
	return c.executable()
}

func (c *aiCommand) home() (string, error) {
	if c.homeDir == nil {
		return "", errors.New("ai home directory resolver is not configured")
	}
	return c.homeDir()
}

func (c *aiCommand) homeOrEmpty() string {
	homeDir, err := c.home()
	if err != nil {
		return ""
	}
	return homeDir
}

func (c *aiCommand) env(name string) string {
	if c.lookupEnv == nil {
		return ""
	}
	return c.lookupEnv(name)
}

func (c *aiCommand) run(name string, args ...string) error {
	if c.runCommand == nil {
		return errors.New("ai command runner is not configured")
	}
	return c.runCommand(context.Background(), name, args...)
}

func (c *aiCommand) read(name string, args ...string) ([]byte, error) {
	if c.readCommand == nil {
		return nil, errors.New("ai command reader is not configured")
	}
	return c.readCommand(context.Background(), name, args...)
}

func (c *aiCommand) readTrimmed(name string, args ...string) string {
	out, err := c.read(name, args...)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func parseAISplitDirection(args []string, command string, stderr io.Writer) (string, error) {
	direction := "right"
	switch len(args) {
	case 0:
	case 1:
		direction = strings.TrimSpace(args[0])
	default:
		printAIUsage(stderr)
		return "", fmt.Errorf("%s accepts at most 1 [right|down] argument", command)
	}
	switch direction {
	case "right", "down":
		return direction, nil
	default:
		printAIUsage(stderr)
		return "", fmt.Errorf("%s direction must be right or down", command)
	}
}

func normalizeAIMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case aiModeClaude, aiModeCodex, aiModeSelective, aiModeShell:
		return strings.TrimSpace(mode)
	default:
		return aiModeSelective
	}
}

func runExternalCommand(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func readExternalCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	if name == "command" && len(args) >= 2 && args[0] == "-v" {
		path, err := exec.LookPath(args[1])
		if err != nil {
			return nil, err
		}
		return []byte(path + "\n"), nil
	}
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func shellEnv(name, value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return name + "=" + shellQuote(value) + " "
}

func ansiDim(value string) string {
	return "\x1b[90m" + value + "\x1b[0m"
}

func isDir(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}

func firstExecutable(paths ...string) string {
	for _, path := range paths {
		if isExecutable(path) {
			return path
		}
	}
	return ""
}

func newestExecutable(paths []string) string {
	var newestPath string
	var newestMod time.Time
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
			continue
		}
		if newestPath == "" || info.ModTime().After(newestMod) {
			newestPath = path
			newestMod = info.ModTime()
		}
	}
	return newestPath
}

func parsePositiveInt(value string) int {
	n := 0
	for _, r := range strings.TrimSpace(value) {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func printAIUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  projmux ai split [right|down]")
	fmt.Fprintln(w, "  projmux ai picker [--inside] [--shell] [right|down]")
	fmt.Fprintln(w, "  projmux ai settings [--get|--set <mode>]")
}
