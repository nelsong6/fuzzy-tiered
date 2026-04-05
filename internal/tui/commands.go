package tui

import "github.com/nelsong6/fzt/internal/model"

// buildCommandItems creates tree items for the command palette.
func buildCommandItems(s *state) []model.Item {
	var items []model.Item

	// "version" folder with on/off children (indices 0, 1, 2)
	items = append(items, model.Item{
		Fields:      []string{"version", "Show/hide version in title bar"},
		Depth:       0,
		HasChildren: true,
		ParentIdx:   -1,
		Children:    []int{1, 2},
	})
	items = append(items, model.Item{
		Fields:    []string{"on", "Show version"},
		Depth:     1,
		ParentIdx: 0,
	})
	items = append(items, model.Item{
		Fields:    []string{"off", "Hide version"},
		Depth:     1,
		ParentIdx: 0,
	})

	// "tree edit" folder with clipboard commands (indices 3, 4, 5)
	items = append(items, model.Item{
		Fields:      []string{"tree edit", "Copy/paste bookmark tree as YAML"},
		Depth:       0,
		HasChildren: true,
		ParentIdx:   -1,
		Children:    []int{4, 5},
	})
	items = append(items, model.Item{
		Fields:    []string{"copy yaml", "Copy bookmark tree to clipboard"},
		Depth:     1,
		ParentIdx: 3,
	})
	items = append(items, model.Item{
		Fields:    []string{"paste yaml", "Save clipboard YAML as bookmarks"},
		Depth:     1,
		ParentIdx: 3,
	})

	return items
}

// newCommandContext creates a treeContext for the command palette.
func newCommandContext(s *state) treeContext {
	cmdItems := buildCommandItems(s)

	// Compute nameColWidth for command items
	nameColW := 0
	for _, item := range cmdItems {
		if len(item.Fields) > 0 {
			w := len([]rune(item.Fields[0]))
			if w > nameColW {
				nameColW = w
			}
		}
	}

	ctx := treeContext{
		allItems:      cmdItems,
		items:         cmdItems,
		nameColWidth:  nameColW,
		colGap:        2,
		index:         -1,
		treeExpanded:  make(map[int]bool),
		queryExpanded: make(map[int]bool),
		treeCursor:    -1,
		scope:         []scopeLevel{{parentIdx: -1}},
		kind:          contextCommand,
		promptIcon:    ':',
		onLeafSelect: func(item model.Item) string {
			if len(item.Fields) == 0 {
				return ""
			}
			name := item.Fields[0]
			var action string
			switch name {
			case "on":
				s.showVersion = true
			case "off":
				s.showVersion = false
			case "copy yaml":
				action = "copy-yaml"
			case "paste yaml":
				action = "paste-yaml"
			}
			popContext(s)
			return action
		},
	}

	return ctx
}
