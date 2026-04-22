package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/es5h/projmux/internal/core/lifecycle"
	coresessions "github.com/es5h/projmux/internal/core/sessions"
	inttmux "github.com/es5h/projmux/internal/integrations/tmux"
)

type killCurrentSessionResolver interface {
	CurrentSessionName(ctx context.Context) (string, error)
}

type killRecentSessionsResolver interface {
	RecentSessions(ctx context.Context) ([]string, error)
}

type taggedKillExecutor interface {
	Execute(ctx context.Context, inputs lifecycle.TaggedKillInputs) (lifecycle.TaggedKillResult, error)
}

type killCommand struct {
	current killCurrentSessionResolver
	recent  killRecentSessionsResolver
	exec    taggedKillExecutor
	homeDir func() (string, error)
}

func newKillCommand() *killCommand {
	client := inttmux.NewClient(inttmux.ExecRunner{})

	return &killCommand{
		current: client,
		recent:  client,
		exec:    lifecycle.NewTaggedKiller(client, client),
		homeDir: os.UserHomeDir,
	}
}

// Run manages kill subcommands.
func (c *killCommand) Run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("kill", flag.ContinueOnError)
	fs.SetOutput(stderr)

	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		printKillUsage(stderr)
		return errors.New("kill requires a subcommand")
	}

	switch fs.Arg(0) {
	case "tagged":
		return c.runTagged(fs.Args()[1:], stdout, stderr)
	case "help", "--help", "-h":
		printKillUsage(stdout)
		return nil
	default:
		printKillUsage(stderr)
		return fmt.Errorf("unknown kill subcommand: %s", fs.Arg(0))
	}
}

func (c *killCommand) runTagged(args []string, _ io.Writer, stderr io.Writer) error {
	targets, err := normalizeTaggedArgs(args, stderr)
	if err != nil {
		return err
	}

	inputs, err := c.taggedInputs(context.Background(), targets)
	if err != nil {
		return err
	}

	if c.exec == nil {
		return fmt.Errorf("kill tagged executor is not configured")
	}

	if _, err := c.exec.Execute(context.Background(), inputs); err != nil {
		return fmt.Errorf("kill tagged sessions: %w", err)
	}

	return nil
}

func (c *killCommand) taggedInputs(ctx context.Context, targets []string) (lifecycle.TaggedKillInputs, error) {
	current, err := c.resolveCurrentSession(ctx)
	if err != nil {
		return lifecycle.TaggedKillInputs{}, err
	}

	recent, err := c.resolveRecentSessions(ctx)
	if err != nil {
		return lifecycle.TaggedKillInputs{}, err
	}

	homeSession, err := c.resolveHomeSession()
	if err != nil {
		return lifecycle.TaggedKillInputs{}, err
	}

	return lifecycle.TaggedKillInputs{
		CurrentSession: current,
		KillTargets:    targets,
		RecentSessions: recent,
		HomeSession:    homeSession,
	}, nil
}

func (c *killCommand) resolveCurrentSession(ctx context.Context) (string, error) {
	if c.current == nil {
		return "", fmt.Errorf("resolve current tmux session: current session resolver is not configured")
	}

	current, err := c.current.CurrentSessionName(ctx)
	if err != nil {
		return "", fmt.Errorf("resolve current tmux session: %w", err)
	}

	return strings.TrimSpace(current), nil
}

func (c *killCommand) resolveRecentSessions(ctx context.Context) ([]string, error) {
	if c.recent == nil {
		return nil, fmt.Errorf("resolve recent tmux sessions: recent session resolver is not configured")
	}

	recent, err := c.recent.RecentSessions(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve recent tmux sessions: %w", err)
	}

	return recent, nil
}

func (c *killCommand) resolveHomeSession() (string, error) {
	if c.homeDir == nil {
		return "", fmt.Errorf("resolve home session identity: home directory resolver is not configured")
	}

	homeDir, err := c.homeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home session identity: %w", err)
	}

	cleanHome := filepath.Clean(homeDir)
	return coresessions.NewNamer(cleanHome).SessionName(cleanHome), nil
}

func normalizeTaggedArgs(args []string, stderr io.Writer) ([]string, error) {
	if len(args) == 0 {
		printKillUsage(stderr)
		return nil, fmt.Errorf("kill tagged requires at least 1 <session> argument")
	}

	targets := make([]string, 0, len(args))
	seen := make(map[string]struct{}, len(args))
	for _, arg := range args {
		target := strings.TrimSpace(arg)
		if target == "" {
			printKillUsage(stderr)
			return nil, fmt.Errorf("kill tagged requires non-empty <session> arguments")
		}
		if _, ok := seen[target]; ok {
			continue
		}

		seen[target] = struct{}{}
		targets = append(targets, target)
	}

	return targets, nil
}

func printKillUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  projmux kill tagged <session>...")
}
