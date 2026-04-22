package preview

import (
	"errors"
	"testing"
)

func TestNextPaneFallsBackToActivePaneWhenStoredSelectionIsMissing(t *testing.T) {
	t.Parallel()

	got, ok, err := NextPane(CycleInputs{
		Panes: []Pane{
			{WindowIndex: "1", Index: "0"},
			{WindowIndex: "1", Index: "1", Active: true},
			{WindowIndex: "1", Index: "2"},
		},
	})
	if err != nil {
		t.Fatalf("NextPane() error = %v", err)
	}
	if !ok {
		t.Fatal("NextPane() ok = false, want true")
	}
	if got != (Cursor{WindowIndex: "1", PaneIndex: "2"}) {
		t.Fatalf("NextPane() = %+v, want window 1 pane 2", got)
	}
}

func TestPreviousPaneWrapsWithinCurrentWindow(t *testing.T) {
	t.Parallel()

	got, ok, err := PreviousPane(CycleInputs{
		StoredWindowIndex: "1",
		StoredPaneIndex:   "0",
		Panes: []Pane{
			{WindowIndex: "1", Index: "0"},
			{WindowIndex: "1", Index: "1"},
			{WindowIndex: "1", Index: "2"},
		},
	})
	if err != nil {
		t.Fatalf("PreviousPane() error = %v", err)
	}
	if !ok {
		t.Fatal("PreviousPane() ok = false, want true")
	}
	if got != (Cursor{WindowIndex: "1", PaneIndex: "2"}) {
		t.Fatalf("PreviousPane() = %+v, want window 1 pane 2", got)
	}
}

func TestNextPaneFallsBackToFirstPaneWhenStoredPaneIsMissing(t *testing.T) {
	t.Parallel()

	got, ok, err := NextPane(CycleInputs{
		StoredWindowIndex: "2",
		StoredPaneIndex:   "9",
		Panes: []Pane{
			{WindowIndex: "1", Index: "0"},
			{WindowIndex: "2", Index: "3"},
			{WindowIndex: "2", Index: "4"},
		},
	})
	if err != nil {
		t.Fatalf("NextPane() error = %v", err)
	}
	if !ok {
		t.Fatal("NextPane() ok = false, want true")
	}
	if got != (Cursor{WindowIndex: "2", PaneIndex: "3"}) {
		t.Fatalf("NextPane() = %+v, want window 2 pane 3", got)
	}
}

func TestNextWindowFallsBackToActiveWindowAndUsesTargetActivePane(t *testing.T) {
	t.Parallel()

	got, ok, err := NextWindow(CycleInputs{
		Windows: []Window{
			{Index: "1"},
			{Index: "2", Active: true},
			{Index: "3"},
		},
		Panes: []Pane{
			{WindowIndex: "2", Index: "4", Active: true},
			{WindowIndex: "3", Index: "7"},
			{WindowIndex: "3", Index: "8", Active: true},
		},
	})
	if err != nil {
		t.Fatalf("NextWindow() error = %v", err)
	}
	if !ok {
		t.Fatal("NextWindow() ok = false, want true")
	}
	if got != (Cursor{WindowIndex: "3", PaneIndex: "8"}) {
		t.Fatalf("NextWindow() = %+v, want window 3 pane 8", got)
	}
}

func TestPreviousWindowWrapsAndFallsBackToFirstPaneWhenTargetHasNoActivePane(t *testing.T) {
	t.Parallel()

	got, ok, err := PreviousWindow(CycleInputs{
		StoredWindowIndex: "1",
		Windows: []Window{
			{Index: "1", Active: true},
			{Index: "2"},
			{Index: "3"},
		},
		Panes: []Pane{
			{WindowIndex: "3", Index: "7"},
			{WindowIndex: "3", Index: "8"},
		},
	})
	if err != nil {
		t.Fatalf("PreviousWindow() error = %v", err)
	}
	if !ok {
		t.Fatal("PreviousWindow() ok = false, want true")
	}
	if got != (Cursor{WindowIndex: "3", PaneIndex: "7"}) {
		t.Fatalf("PreviousWindow() = %+v, want window 3 pane 7", got)
	}
}

func TestNextWindowFallsBackToFirstWindowWhenStoredWindowIsMissing(t *testing.T) {
	t.Parallel()

	got, ok, err := NextWindow(CycleInputs{
		StoredWindowIndex: "9",
		Windows: []Window{
			{Index: "1"},
			{Index: "2", Active: true},
		},
		Panes: []Pane{
			{WindowIndex: "1", Index: "0"},
			{WindowIndex: "2", Index: "5", Active: true},
		},
	})
	if err != nil {
		t.Fatalf("NextWindow() error = %v", err)
	}
	if !ok {
		t.Fatal("NextWindow() ok = false, want true")
	}
	if got != (Cursor{WindowIndex: "1", PaneIndex: "0"}) {
		t.Fatalf("NextWindow() = %+v, want window 1 pane 0", got)
	}
}

func TestCycleHelpersRejectInvalidDirection(t *testing.T) {
	t.Parallel()

	_, _, err := cyclePane(CycleInputs{}, Direction("sideways"))
	if !errors.Is(err, ErrInvalidDirection) {
		t.Fatalf("cyclePane() error = %v, want %v", err, ErrInvalidDirection)
	}

	_, _, err = cycleWindow(CycleInputs{}, Direction("sideways"))
	if !errors.Is(err, ErrInvalidDirection) {
		t.Fatalf("cycleWindow() error = %v, want %v", err, ErrInvalidDirection)
	}
}
