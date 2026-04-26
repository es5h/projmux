package picker

import "strings"

// Item is the backend-neutral representation of a selectable picker row.
type Item struct {
	Title         string
	Value         string
	SearchText    string
	MetaLines     []string
	Badges        []string
	PreviewTarget string
}

func (i Item) EffectiveSearchText() string {
	if search := strings.TrimSpace(i.SearchText); search != "" {
		return search
	}
	return strings.TrimSpace(i.Title)
}
