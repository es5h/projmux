package picker

import "testing"

func TestItemEffectiveSearchTextPrefersExplicitSearchText(t *testing.T) {
	t.Parallel()

	item := Item{
		Title:      "project-title",
		SearchText: "project alias",
	}

	if got, want := item.EffectiveSearchText(), "project alias"; got != want {
		t.Fatalf("EffectiveSearchText() = %q, want %q", got, want)
	}
}

func TestItemEffectiveSearchTextFallsBackToTitle(t *testing.T) {
	t.Parallel()

	item := Item{Title: " project-title "}

	if got, want := item.EffectiveSearchText(), "project-title"; got != want {
		t.Fatalf("EffectiveSearchText() = %q, want %q", got, want)
	}
}
