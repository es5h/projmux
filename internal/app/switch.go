package app

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/es5h/projmux/internal/config"
	"github.com/es5h/projmux/internal/core/candidates"
	"github.com/es5h/projmux/internal/core/pins"
	inttmux "github.com/es5h/projmux/internal/integrations/tmux"
	intfzf "github.com/es5h/projmux/internal/ui/fzf"
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

type switchRunner interface {
	Run(options intfzf.Options) (string, error)
}

type switchSessionExecutor interface {
	EnsureSession(ctx context.Context, sessionName, cwd string) error
	OpenSession(ctx context.Context, sessionName string) error
}

type switchCommand struct {
	discover    candidateDiscoverer
	pinStore    switchPinStoreFactory
	runner      switchRunner
	sessions    switchSessionExecutor
	identity    sessionIdentityResolver
	identityErr error
	validate    func(path string) error
	homeDir     func() (string, error)
	workingDir  func() (string, error)
	lookupEnv   func(string) string
}

type switchPlan struct {
	UI          string
	Candidates  []string
	Selection   string
	SessionName string
}

func newSwitchCommand() *switchCommand {
	client := inttmux.NewClient(inttmux.ExecRunner{})
	identity, err := newDefaultCurrentIdentityResolver()

	return &switchCommand{
		discover:    candidates.Discover,
		pinStore:    newDefaultSwitchPinStore,
		runner:      intfzf.NewRunner(),
		sessions:    client,
		identity:    identity,
		identityErr: err,
		validate:    validateDirectory,
		homeDir:     os.UserHomeDir,
		workingDir:  os.Getwd,
		lookupEnv:   os.Getenv,
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

// Run resolves the first sessionizer candidate list and opens the first
// interactive picker surface.
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

	if err := c.execute(context.Background(), plan); err != nil {
		return err
	}

	return nil
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

	plan := switchPlan{
		UI:         ui,
		Candidates: paths,
	}

	return c.completePlan(plan)
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

func (c *switchCommand) completePlan(plan switchPlan) (switchPlan, error) {
	if c.runner == nil {
		return switchPlan{}, fmt.Errorf("switch picker is not configured")
	}

	selection, err := c.runPicker(plan)
	if err != nil {
		return switchPlan{}, err
	}
	selection = cleanOptionalPath(selection)
	plan.Selection = selection
	if selection == "" {
		return plan, nil
	}

	if c.validate == nil {
		return switchPlan{}, fmt.Errorf("switch directory validator is not configured")
	}
	if err := c.validate(selection); err != nil {
		return switchPlan{}, err
	}

	if c.identityErr != nil {
		return switchPlan{}, fmt.Errorf("configure session identity resolver: %w", c.identityErr)
	}
	if c.identity == nil {
		return switchPlan{}, fmt.Errorf("switch session identity resolver is not configured")
	}

	sessionName, err := c.identity.SessionIdentityForPath(selection)
	if err != nil {
		return switchPlan{}, fmt.Errorf("resolve session identity: %w", err)
	}
	plan.SessionName = sessionName

	return plan, nil
}

func (c *switchCommand) execute(ctx context.Context, plan switchPlan) error {
	if plan.Selection == "" {
		return nil
	}
	if plan.SessionName == "" {
		return fmt.Errorf("switch command requires a target session")
	}
	if c.sessions == nil {
		return fmt.Errorf("switch session executor is not configured")
	}

	if err := c.sessions.EnsureSession(ctx, plan.SessionName, plan.Selection); err != nil {
		return fmt.Errorf("ensure tmux session %q: %w", plan.SessionName, err)
	}
	if err := c.sessions.OpenSession(ctx, plan.SessionName); err != nil {
		return fmt.Errorf("open tmux session %q: %w", plan.SessionName, err)
	}

	return nil
}

func (c *switchCommand) runPicker(plan switchPlan) (string, error) {
	if c.runner == nil {
		return "", fmt.Errorf("switch runner is not configured")
	}

	selection, err := c.runner.Run(intfzf.Options{
		UI:         plan.UI,
		Candidates: plan.Candidates,
	})
	if err != nil {
		return "", fmt.Errorf("run switch picker: %w", err)
	}

	return selection, nil
}

func printSwitchUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: projmux switch [--ui=popup|sidebar]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  --ui string   Candidate surface to prepare (popup or sidebar) (default \"popup\")")
}
