package app

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/es5h/projmux/internal/config"
	"github.com/es5h/projmux/internal/core/candidates"
	"github.com/es5h/projmux/internal/core/pins"
)

const (
	switchUIFlag             = "ui"
	switchUIPopup            = "popup"
	switchUISidebar          = "sidebar"
	managedRootsEnvVar       = "PROJMUX_MANAGED_ROOTS"
	legacyManagedRootsEnvVar = "TMUX_SESSIONIZER_ROOTS"
	repoRootEnvVar           = "RP"
)

type candidateDiscoverer func(inputs candidates.Inputs) ([]string, error)

type switchPinStore interface {
	List() ([]string, error)
}

type switchPinStoreFactory func() (switchPinStore, error)

type switchCommand struct {
	discover   candidateDiscoverer
	pinStore   switchPinStoreFactory
	homeDir    func() (string, error)
	workingDir func() (string, error)
	lookupEnv  func(string) string
}

type switchPlan struct {
	UI         string
	Candidates []string
}

func newSwitchCommand() *switchCommand {
	return &switchCommand{
		discover:   candidates.Discover,
		pinStore:   newDefaultSwitchPinStore,
		homeDir:    os.UserHomeDir,
		workingDir: os.Getwd,
		lookupEnv:  os.Getenv,
	}
}

func newDefaultSwitchPinStore() (switchPinStore, error) {
	paths, err := config.DefaultPathsFromEnv()
	if err != nil {
		return nil, err
	}

	store := pins.NewDefaultStore(paths)
	return store, nil
}

// Run resolves the first sessionizer candidate list and prints it in a simple
// inspectable text form.
func (c *switchCommand) Run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("switch", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		printSwitchUsage(stderr)
	}

	ui := fs.String(switchUIFlag, switchUIPopup, "future sessionizer surface to prepare")
	if err := fs.Parse(args); err != nil {
		printSwitchUsage(stderr)
		return err
	}
	if fs.NArg() != 0 {
		printSwitchUsage(stderr)
		return fmt.Errorf("switch does not accept positional arguments")
	}
	if err := validateSwitchUI(*ui); err != nil {
		printSwitchUsage(stderr)
		return err
	}

	plan, err := c.plan(*ui)
	if err != nil {
		return err
	}

	return renderSwitchPlan(stdout, plan)
}

func (c *switchCommand) plan(ui string) (switchPlan, error) {
	homeDir, err := c.resolveHomeDir()
	if err != nil {
		return switchPlan{}, err
	}

	pins, err := c.loadPins()
	if err != nil {
		return switchPlan{}, err
	}

	currentPath, err := c.resolveWorkingDir()
	if err != nil {
		return switchPlan{}, err
	}

	if c.discover == nil {
		return switchPlan{}, fmt.Errorf("switch candidate discovery is not configured")
	}

	paths, err := c.discover(candidates.Inputs{
		HomeDir:      homeDir,
		RepoRoot:     cleanOptionalPath(c.env(repoRootEnvVar)),
		ManagedRoots: switchManagedRootsFromEnv(c.lookupEnv),
		Pins:         pins,
		CurrentPath:  currentPath,
	})
	if err != nil {
		return switchPlan{}, fmt.Errorf("discover switch candidates: %w", err)
	}

	return switchPlan{
		UI:         ui,
		Candidates: paths,
	}, nil
}

func (c *switchCommand) resolveHomeDir() (string, error) {
	if c.homeDir == nil {
		return "", fmt.Errorf("switch home directory resolver is not configured")
	}

	homeDir, err := c.homeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	return filepath.Clean(homeDir), nil
}

func (c *switchCommand) loadPins() ([]string, error) {
	if c.pinStore == nil {
		return nil, nil
	}

	store, err := c.pinStore()
	if err != nil {
		return nil, fmt.Errorf("configure pin store: %w", err)
	}
	if store == nil {
		return nil, nil
	}

	paths, err := store.List()
	if err != nil {
		return nil, fmt.Errorf("load pin set: %w", err)
	}

	return paths, nil
}

func (c *switchCommand) resolveWorkingDir() (string, error) {
	if c.workingDir == nil {
		return "", fmt.Errorf("switch working directory resolver is not configured")
	}

	path, err := c.workingDir()
	if err != nil {
		return "", fmt.Errorf("resolve current working directory: %w", err)
	}

	return filepath.Clean(path), nil
}

func (c *switchCommand) env(name string) string {
	if c.lookupEnv == nil {
		return ""
	}
	return c.lookupEnv(name)
}

func switchManagedRootsFromEnv(lookup func(string) string) []string {
	roots := make([]string, 0)
	seen := make(map[string]struct{})

	for _, value := range []string{
		envValue(lookup, managedRootsEnvVar),
		envValue(lookup, legacyManagedRootsEnvVar),
	} {
		for _, root := range filepath.SplitList(value) {
			root = cleanOptionalPath(root)
			if root == "" {
				continue
			}
			if _, ok := seen[root]; ok {
				continue
			}
			seen[root] = struct{}{}
			roots = append(roots, root)
		}
	}

	return roots
}

func envValue(lookup func(string) string, name string) string {
	if lookup == nil {
		return ""
	}
	return lookup(name)
}

func cleanOptionalPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	return filepath.Clean(path)
}

func validateSwitchUI(ui string) error {
	switch ui {
	case switchUIPopup, switchUISidebar:
		return nil
	default:
		return fmt.Errorf("invalid --ui value %q: expected %q or %q", ui, switchUIPopup, switchUISidebar)
	}
}

func renderSwitchPlan(w io.Writer, plan switchPlan) error {
	if _, err := fmt.Fprintf(w, "ui: %s\n", plan.UI); err != nil {
		return err
	}

	for i, candidate := range plan.Candidates {
		if _, err := fmt.Fprintf(w, "%d: %s\n", i+1, candidate); err != nil {
			return err
		}
	}

	return nil
}

func printSwitchUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: projmux switch [--ui=popup|sidebar]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  --ui string   Candidate surface to prepare (popup or sidebar) (default \"popup\")")
}
