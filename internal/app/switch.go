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

	"github.com/es5h/projmux/internal/config"
	"github.com/es5h/projmux/internal/core/candidates"
	"github.com/es5h/projmux/internal/core/pins"
	corepreview "github.com/es5h/projmux/internal/core/preview"
	coretags "github.com/es5h/projmux/internal/core/tags"
	inttmux "github.com/es5h/projmux/internal/integrations/tmux"
	intfzf "github.com/es5h/projmux/internal/ui/fzf"
	intrender "github.com/es5h/projmux/internal/ui/render"
)

const (
	switchUIFlag             = "ui"
	switchUIPopup            = "popup"
	switchUISidebar          = "sidebar"
	switchTagExpectKey       = "alt-t"
	managedRootsEnvVar       = "PROJMUX_MANAGED_ROOTS"
	legacyManagedRootsEnvVar = "TMUX_SESSIONIZER_ROOTS"
	repoRootEnvVar           = "RP"
)

type candidateDiscoverer func(inputs candidates.Inputs) ([]string, error)

type switchPinStore interface {
	List() ([]string, error)
}

type switchPinStoreFactory func() (switchPinStore, error)

type switchTagStore interface {
	Toggle(name string) (bool, error)
}

type switchTagStoreFactory func() (switchTagStore, error)

type switchRunner interface {
	Run(options intfzf.Options) (intfzf.Result, error)
}

type switchSessionExecutor interface {
	EnsureSession(ctx context.Context, sessionName, cwd string) error
	OpenSession(ctx context.Context, sessionName string) error
}

type switchSessionInspector interface {
	SessionExists(ctx context.Context, sessionName string) (bool, error)
}

type switchPreviewStore interface {
	ReadSelection(sessionName string) (corepreview.Selection, bool, error)
}

type switchCommand struct {
	discover        candidateDiscoverer
	pinStore        switchPinStoreFactory
	tagStore        switchTagStoreFactory
	runner          switchRunner
	sessions        switchSessionExecutor
	previewStore    switchPreviewStore
	previewStoreErr error
	inventory       previewInventory
	inventoryErr    error
	executable      func() (string, error)
	identity        sessionIdentityResolver
	identityErr     error
	validate        func(path string) error
	homeDir         func() (string, error)
	workingDir      func() (string, error)
	lookupEnv       func(string) string
}

type switchPlan struct {
	UI          string
	Candidates  []string
	Rows        []intfzf.Entry
	Action      string
	Selection   string
	SessionName string
}

func newSwitchCommand() *switchCommand {
	client := inttmux.NewClient(inttmux.ExecRunner{})
	identity, err := newDefaultCurrentIdentityResolver()
	paths, pathsErr := config.DefaultPathsFromEnv()

	cmd := &switchCommand{
		discover:    candidates.Discover,
		pinStore:    newDefaultSwitchPinStore,
		tagStore:    newDefaultSwitchTagStore,
		runner:      intfzf.NewRunner(),
		sessions:    client,
		inventory:   tmuxPreviewInventory{client: client},
		executable:  os.Executable,
		identity:    identity,
		identityErr: err,
		validate:    validateDirectory,
		homeDir:     os.UserHomeDir,
		workingDir:  os.Getwd,
		lookupEnv:   os.Getenv,
	}
	if pathsErr != nil {
		cmd.previewStoreErr = fmt.Errorf("resolve default config paths: %w", pathsErr)
		return cmd
	}
	cmd.previewStore = corepreview.NewDefaultStore(paths)
	return cmd
}

func newDefaultSwitchPinStore() (switchPinStore, error) {
	paths, err := config.DefaultPathsFromEnv()
	if err != nil {
		return nil, err
	}

	store := pins.NewDefaultStore(paths)
	return store, nil
}

func newDefaultSwitchTagStore() (switchTagStore, error) {
	paths, err := config.DefaultPathsFromEnv()
	if err != nil {
		return nil, err
	}

	return coretags.NewDefaultStore(paths), nil
}

// Run resolves the first sessionizer candidate list and opens the first
// interactive picker surface.
func (c *switchCommand) Run(args []string, stdout, stderr io.Writer) error {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		switch args[0] {
		case "toggle-tag":
			return c.runToggleTag(args[1:], stdout, stderr)
		case "preview":
			return c.runPreview(args[1:], stdout, stderr)
		}
	}

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

	ctx := context.Background()
	for {
		plan, err := c.plan(*ui)
		if err != nil {
			return err
		}

		reopen, err := c.execute(ctx, plan, stdout)
		if err != nil {
			return err
		}
		if !reopen {
			return nil
		}
	}
}

func (c *switchCommand) plan(ui string) (switchPlan, error) {
	inputs, err := c.candidateInputs("")
	if err != nil {
		return switchPlan{}, err
	}

	return c.planFromInputs(ui, inputs)
}

func (c *switchCommand) runToggleTag(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("switch toggle-tag", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		printSwitchUsage(stderr)
	}

	if err := fs.Parse(args); err != nil {
		printSwitchUsage(stderr)
		return err
	}
	if fs.NArg() > 1 {
		printSwitchUsage(stderr)
		return fmt.Errorf("switch toggle-tag accepts at most 1 [path] argument")
	}

	target, err := c.resolveToggleTagTarget(fs.Args())
	if err != nil {
		if strings.Contains(err.Error(), "switch toggle-tag requires") {
			printSwitchUsage(stderr)
		}
		return err
	}

	return c.toggleTag(target, stdout)
}

func (c *switchCommand) runPreview(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("switch preview", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		printSwitchUsage(stderr)
	}

	if err := fs.Parse(args); err != nil {
		printSwitchUsage(stderr)
		return err
	}
	if fs.NArg() > 1 {
		printSwitchUsage(stderr)
		return fmt.Errorf("switch preview accepts at most 1 [path] argument")
	}

	target, err := c.resolveSwitchTarget(fs.Args(), "switch preview")
	if err != nil {
		if strings.Contains(err.Error(), "switch preview requires") {
			printSwitchUsage(stderr)
		}
		return err
	}

	model, err := c.previewModel(context.Background(), target)
	if err != nil {
		return err
	}

	_, err = io.WriteString(stdout, intrender.RenderSwitchPreview(model))
	return err
}

func (c *switchCommand) planFromInputs(ui string, inputs candidates.Inputs) (switchPlan, error) {
	homeDir, err := c.resolveHomeDir()
	if err != nil {
		return switchPlan{}, err
	}

	if c.discover == nil {
		return switchPlan{}, fmt.Errorf("switch candidate discovery is not configured")
	}

	if inputs.HomeDir == "" {
		inputs.HomeDir = homeDir
	}

	paths, err := c.discover(inputs)
	if err != nil {
		return switchPlan{}, fmt.Errorf("discover switch candidates: %w", err)
	}

	plan := switchPlan{
		UI:         ui,
		Candidates: paths,
	}

	return c.completePlan(plan)
}

func (c *switchCommand) candidateInputs(currentPath string) (candidates.Inputs, error) {
	homeDir, err := c.resolveHomeDir()
	if err != nil {
		return candidates.Inputs{}, err
	}

	pins, err := c.loadPins()
	if err != nil {
		return candidates.Inputs{}, err
	}

	if currentPath == "" {
		currentPath, err = c.resolveWorkingDir()
		if err != nil {
			return candidates.Inputs{}, err
		}
	}

	return candidates.Inputs{
		HomeDir:      homeDir,
		RepoRoot:     cleanOptionalPath(c.env(repoRootEnvVar)),
		ManagedRoots: switchManagedRoots(homeDir, c.env(repoRootEnvVar), c.lookupEnv),
		Pins:         pins,
		CurrentPath:  currentPath,
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

func (c *switchCommand) resolveSwitchTarget(args []string, command string) (string, error) {
	inputPath, err := c.resolveSwitchInput(args, command)
	if err != nil {
		return "", err
	}

	inputs, err := c.candidateInputs(inputPath)
	if err != nil {
		return "", err
	}
	if c.discover == nil {
		return "", fmt.Errorf("switch candidate discovery is not configured")
	}

	candidatePaths, err := c.discover(inputs)
	if err != nil {
		return "", fmt.Errorf("discover switch candidates: %w", err)
	}

	target := bestSwitchCandidateMatch(inputPath, candidatePaths)
	if target == "" {
		return "", fmt.Errorf("resolve switch tag target: no switch candidate matched %q", inputPath)
	}

	return target, nil
}

func (c *switchCommand) resolveToggleTagTarget(args []string) (string, error) {
	return c.resolveSwitchTarget(args, "switch toggle-tag")
}

func (c *switchCommand) resolveSwitchInput(args []string, command string) (string, error) {
	var path string

	switch len(args) {
	case 0:
		var err error
		path, err = c.resolveWorkingDir()
		if err != nil {
			return "", err
		}
	case 1:
		if strings.TrimSpace(args[0]) == "" {
			return "", fmt.Errorf("%s requires a non-empty [path] argument", command)
		}
		path = args[0]
	default:
		return "", fmt.Errorf("%s accepts at most 1 [path] argument", command)
	}

	if !filepath.IsAbs(path) {
		workingDir, err := c.resolveWorkingDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(workingDir, path)
	}
	path = filepath.Clean(path)

	if c.validate == nil {
		return "", fmt.Errorf("switch directory validator is not configured")
	}
	if err := c.validate(path); err != nil {
		return "", fmt.Errorf("validate switch tag path %q: %w", path, err)
	}

	return path, nil
}

func (c *switchCommand) previewModel(ctx context.Context, target string) (corepreview.SwitchReadModel, error) {
	homeDir, err := c.resolveHomeDir()
	if err != nil {
		return corepreview.SwitchReadModel{}, err
	}
	repoRoot := cleanOptionalPath(c.env(repoRootEnvVar))

	if c.identityErr != nil {
		return corepreview.SwitchReadModel{}, fmt.Errorf("configure session identity resolver: %w", c.identityErr)
	}
	if c.identity == nil {
		return corepreview.SwitchReadModel{}, fmt.Errorf("switch session identity resolver is not configured")
	}

	sessionName, err := c.identity.SessionIdentityForPath(target)
	if err != nil {
		return corepreview.SwitchReadModel{}, fmt.Errorf("resolve switch preview session identity: %w", err)
	}

	modelInputs := corepreview.SwitchReadModelInputs{
		Path:        target,
		DisplayPath: intrender.PrettyPath(target, homeDir, repoRoot),
		SessionName: sessionName,
	}

	exists, err := c.switchSessionExists(ctx, sessionName)
	if err != nil {
		return corepreview.SwitchReadModel{}, err
	}
	modelInputs.SessionExists = exists
	if !exists {
		return corepreview.BuildSwitchReadModel(modelInputs), nil
	}

	store, err := c.requireSwitchPreviewStore()
	if err != nil {
		return corepreview.SwitchReadModel{}, err
	}
	inventory, err := c.requireSwitchPreviewInventory()
	if err != nil {
		return corepreview.SwitchReadModel{}, err
	}

	selection, hasSelection, err := store.ReadSelection(sessionName)
	if err != nil {
		return corepreview.SwitchReadModel{}, fmt.Errorf("load switch preview selection for %q: %w", sessionName, err)
	}
	windows, err := inventory.SessionWindows(ctx, sessionName)
	if err != nil {
		return corepreview.SwitchReadModel{}, fmt.Errorf("load switch preview windows for %q: %w", sessionName, err)
	}
	panes, err := inventory.SessionPanes(ctx, sessionName)
	if err != nil {
		return corepreview.SwitchReadModel{}, fmt.Errorf("load switch preview panes for %q: %w", sessionName, err)
	}

	modelInputs.StoredSelection = selection
	modelInputs.HasStoredSelection = hasSelection
	modelInputs.Windows = windows
	modelInputs.Panes = panes
	return corepreview.BuildSwitchReadModel(modelInputs), nil
}

func (c *switchCommand) switchSessionExists(ctx context.Context, sessionName string) (bool, error) {
	inspector, ok := c.sessions.(switchSessionInspector)
	if !ok || inspector == nil {
		return false, nil
	}

	exists, err := inspector.SessionExists(ctx, sessionName)
	if err != nil {
		return false, fmt.Errorf("check existing switch session %q: %w", sessionName, err)
	}
	return exists, nil
}

func (c *switchCommand) requireSwitchPreviewStore() (switchPreviewStore, error) {
	if c.previewStoreErr != nil {
		return nil, fmt.Errorf("configure switch preview store: %w", c.previewStoreErr)
	}
	if c.previewStore == nil {
		return nil, errors.New("configure switch preview store: switch preview store is not configured")
	}
	return c.previewStore, nil
}

func (c *switchCommand) requireSwitchPreviewInventory() (previewInventory, error) {
	if c.inventoryErr != nil {
		return nil, fmt.Errorf("configure switch preview inventory: %w", c.inventoryErr)
	}
	if c.inventory == nil {
		return nil, errors.New("configure switch preview inventory: switch preview inventory is not configured")
	}
	return c.inventory, nil
}

func (c *switchCommand) loadTagStore() (switchTagStore, error) {
	if c.tagStore == nil {
		return nil, errors.New("configure switch tag store: tag store is not configured")
	}

	store, err := c.tagStore()
	if err != nil {
		return nil, fmt.Errorf("configure switch tag store: %w", err)
	}
	if store == nil {
		return nil, errors.New("configure switch tag store: tag store is not configured")
	}

	return store, nil
}

func (c *switchCommand) env(name string) string {
	if c.lookupEnv == nil {
		return ""
	}
	return c.lookupEnv(name)
}

func switchManagedRoots(homeDir, repoRoot string, lookup func(string) string) []string {
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

	if len(roots) == 0 {
		for _, root := range defaultManagedRoots(homeDir, repoRoot) {
			if _, ok := seen[root]; ok {
				continue
			}
			seen[root] = struct{}{}
			roots = append(roots, root)
		}
	}

	return roots
}

func defaultManagedRoots(homeDir, repoRoot string) []string {
	roots := make([]string, 0, 6)
	for _, root := range []string{
		filepath.Join(homeDir, "source"),
		filepath.Join(homeDir, "work"),
		filepath.Join(homeDir, "projects"),
		filepath.Join(homeDir, "src"),
		filepath.Join(homeDir, "code"),
		repoRoot,
	} {
		root = cleanOptionalPath(root)
		if root == "" {
			continue
		}
		roots = append(roots, root)
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

func bestSwitchCandidateMatch(path string, candidatePaths []string) string {
	cleanPath := filepath.Clean(path)
	best := ""

	for _, candidatePath := range candidatePaths {
		candidatePath = filepath.Clean(candidatePath)
		if candidatePath == cleanPath {
			return candidatePath
		}

		prefix := candidatePath + string(filepath.Separator)
		if !strings.HasPrefix(cleanPath, prefix) {
			continue
		}
		if len(candidatePath) > len(best) {
			best = candidatePath
		}
	}

	return best
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
	if c.identityErr != nil {
		return switchPlan{}, fmt.Errorf("configure session identity resolver: %w", c.identityErr)
	}
	if c.identity == nil {
		return switchPlan{}, fmt.Errorf("switch session identity resolver is not configured")
	}

	rows, err := c.renderRows(context.Background(), plan.Candidates)
	if err != nil {
		return switchPlan{}, err
	}
	plan.Rows = rows

	result, err := c.runPicker(plan)
	if err != nil {
		return switchPlan{}, err
	}
	plan.Action = strings.TrimSpace(result.Key)
	selection := result.Value
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

	if plan.Action != switchTagExpectKey {
		sessionName, err := c.identity.SessionIdentityForPath(selection)
		if err != nil {
			return switchPlan{}, fmt.Errorf("resolve session identity: %w", err)
		}
		plan.SessionName = sessionName
	}

	return plan, nil
}

func (c *switchCommand) execute(ctx context.Context, plan switchPlan, stdout io.Writer) (bool, error) {
	if plan.Selection == "" {
		return false, nil
	}
	if plan.Action == switchTagExpectKey {
		if err := c.toggleTag(plan.Selection, nil); err != nil {
			return false, err
		}
		return true, nil
	}
	if plan.SessionName == "" {
		return false, fmt.Errorf("switch command requires a target session")
	}
	if c.sessions == nil {
		return false, fmt.Errorf("switch session executor is not configured")
	}

	if err := c.sessions.EnsureSession(ctx, plan.SessionName, plan.Selection); err != nil {
		return false, fmt.Errorf("ensure tmux session %q: %w", plan.SessionName, err)
	}
	if err := c.sessions.OpenSession(ctx, plan.SessionName); err != nil {
		return false, fmt.Errorf("open tmux session %q: %w", plan.SessionName, err)
	}

	return false, nil
}

func (c *switchCommand) runPicker(plan switchPlan) (intfzf.Result, error) {
	if c.runner == nil {
		return intfzf.Result{}, fmt.Errorf("switch runner is not configured")
	}

	options := intfzf.Options{
		UI:         plan.UI,
		Candidates: plan.Candidates,
		Entries:    plan.Rows,
		ExpectKeys: []string{switchTagExpectKey},
	}
	if previewCommand, err := c.switchPreviewCommand(); err != nil {
		return intfzf.Result{}, err
	} else if previewCommand != "" {
		options.PreviewCommand = previewCommand
		options.PreviewWindow = switchPreviewWindow(plan.UI)
	}

	result, err := c.runner.Run(options)
	if err != nil {
		return intfzf.Result{}, fmt.Errorf("run switch picker: %w", err)
	}

	return result, nil
}

func (c *switchCommand) switchPreviewCommand() (string, error) {
	if c.executable == nil {
		return "", nil
	}

	binaryPath, err := c.executable()
	if err != nil {
		return "", fmt.Errorf("resolve switch preview executable: %w", err)
	}

	command, err := inttmux.BuildSwitchPreviewCommand(binaryPath)
	if err != nil {
		return "", fmt.Errorf("build switch preview command: %w", err)
	}
	return command, nil
}

func switchPreviewWindow(ui string) string {
	switch ui {
	case switchUISidebar:
		return "right,60%,border-left"
	case switchUIPopup:
		return "down,35%,border-top"
	default:
		return ""
	}
}

func (c *switchCommand) toggleTag(target string, stdout io.Writer) error {
	store, err := c.loadTagStore()
	if err != nil {
		return err
	}

	tagged, err := store.Toggle(target)
	if err != nil {
		return fmt.Errorf("toggle switch candidate tag: %w", err)
	}
	if stdout == nil {
		return nil
	}

	if tagged {
		_, err = fmt.Fprintf(stdout, "tagged: %s\n", target)
		return err
	}

	_, err = fmt.Fprintf(stdout, "untagged: %s\n", target)
	return err
}

func (c *switchCommand) renderRows(ctx context.Context, candidatePaths []string) ([]intfzf.Entry, error) {
	renderCandidates := make([]intrender.SwitchCandidate, 0, len(candidatePaths))
	existingBySession, err := c.lookupExistingSessions(ctx, candidatePaths)
	if err != nil {
		return nil, err
	}
	homeDir, err := c.resolveHomeDir()
	if err != nil {
		return nil, err
	}
	repoRoot := cleanOptionalPath(c.env(repoRootEnvVar))

	for _, candidatePath := range candidatePaths {
		sessionName, err := c.identity.SessionIdentityForPath(candidatePath)
		if err != nil {
			return nil, fmt.Errorf("render switch rows: resolve session identity for %q: %w", candidatePath, err)
		}

		modeLabel := ""
		if exists, ok := existingBySession[sessionName]; ok {
			if exists {
				modeLabel = "existing"
			} else {
				modeLabel = "new"
			}
		}

		renderCandidates = append(renderCandidates, intrender.SwitchCandidate{
			Path:        candidatePath,
			DisplayPath: intrender.PrettyPath(candidatePath, homeDir, repoRoot),
			SessionName: sessionName,
			ModeLabel:   modeLabel,
		})
	}

	rows := intrender.BuildSwitchRows(renderCandidates)
	entries := make([]intfzf.Entry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, intfzf.Entry{
			Label: row.Label,
			Value: row.Value,
		})
	}

	return entries, nil
}

func (c *switchCommand) lookupExistingSessions(ctx context.Context, candidatePaths []string) (map[string]bool, error) {
	inspector, ok := c.sessions.(switchSessionInspector)
	if !ok || inspector == nil {
		return nil, nil
	}

	existingBySession := make(map[string]bool)
	for _, candidatePath := range candidatePaths {
		sessionName, err := c.identity.SessionIdentityForPath(candidatePath)
		if err != nil {
			return nil, fmt.Errorf("check existing switch sessions: resolve session identity for %q: %w", candidatePath, err)
		}
		if _, seen := existingBySession[sessionName]; seen {
			continue
		}

		exists, err := inspector.SessionExists(ctx, sessionName)
		if err != nil {
			return nil, fmt.Errorf("check existing switch sessions for %q: %w", sessionName, err)
		}
		existingBySession[sessionName] = exists
	}

	return existingBySession, nil
}

func printSwitchUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  projmux switch [--ui=popup|sidebar]")
	fmt.Fprintln(w, "  projmux switch toggle-tag [path]")
	fmt.Fprintln(w, "  projmux switch preview [path]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  --ui string   Candidate surface to prepare (popup or sidebar) (default \"popup\")")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Picker Actions:")
	fmt.Fprintln(w, "  alt-t         Toggle a tag on the focused candidate and reopen the picker")
}
