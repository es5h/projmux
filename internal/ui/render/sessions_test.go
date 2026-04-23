package render

import (
	"testing"

	inttmux "github.com/es5h/projmux/internal/integrations/tmux"
)

func TestBuildSessionRowsSanitizesNames(t *testing.T) {
	t.Parallel()

	rows := BuildSessionRows([]inttmux.RecentSessionSummary{
		{Name: "repo-a", Attached: true, PaneCount: 3, Path: "/tmp/repo-a"},
		{Name: "bad\tname\nx", Attached: false, PaneCount: 1, Path: "/tmp/bad\tpath\nx"},
	})
	want := []SessionRow{
		{Label: "repo-a  [attached]  3p  /tmp/repo-a", Value: "repo-a"},
		{Label: "bad name x  [detached]  1p  /tmp/bad path x", Value: "bad name x"},
	}

	if len(rows) != len(want) {
		t.Fatalf("rows len = %d, want %d", len(rows), len(want))
	}
	for i := range rows {
		if rows[i] != want[i] {
			t.Fatalf("row[%d] = %#v, want %#v", i, rows[i], want[i])
		}
	}
}
