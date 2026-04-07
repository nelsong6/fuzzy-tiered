package core

import (
	"path/filepath"
	"strings"
)

// ExpandToPath pre-expands the tree along the given filesystem path.
// For each segment, it loads children (via Provider if set) and expands the folder.
// The final segment's children are loaded so the user lands inside the target directory.
func ExpandToPath(s *State, targetPath string, cfg Config, searchCols []int) {
	ctx := s.TopCtx()
	if ctx.TreeExpanded == nil {
		ctx.TreeExpanded = make(map[int]bool)
	}

	// Normalize and split path
	targetPath = filepath.Clean(targetPath)
	segments := splitPath(targetPath)
	if len(segments) == 0 {
		return
	}

	// Walk the tree, expanding each segment
	parentIdx := -1
	for _, seg := range segments {
		// Find the matching child
		childIdx := findChild(ctx.AllItems, parentIdx, seg)
		if childIdx < 0 {
			break
		}

		// Load children if needed (lazy provider)
		if len(ctx.AllItems[childIdx].Children) == 0 && s.Provider != nil {
			path := ItemFullPath(ctx, childIdx)
			newItems := s.Provider.LoadChildren(path)
			SpliceChildren(ctx, childIdx, newItems)
		}

		// Expand the folder
		ctx.TreeExpanded[childIdx] = true
		parentIdx = childIdx
	}

	// Update items to reflect the expanded state
	ctx.Items = RootItemsOf(ctx.AllItems)
}

// splitPath splits a filesystem path into its components.
// "D:\repos\my-homepage" → ["D:", "repos", "my-homepage"]
// "/home/user" → ["/", "home", "user"]
func splitPath(p string) []string {
	var parts []string
	for {
		dir, file := filepath.Split(p)
		if file != "" {
			parts = append([]string{file}, parts...)
		}
		if dir == p {
			// Root reached
			dir = strings.TrimRight(dir, string(filepath.Separator))
			if dir != "" {
				parts = append([]string{dir}, parts...)
			} else if filepath.Separator == '/' {
				parts = append([]string{"/"}, parts...)
			}
			break
		}
		p = strings.TrimRight(dir, string(filepath.Separator))
	}
	return parts
}

// findChild finds a child of parentIdx whose name matches seg.
func findChild(allItems []Item, parentIdx int, name string) int {
	if parentIdx < 0 {
		// Search root items
		for i, item := range allItems {
			if item.Depth == 0 && len(item.Fields) > 0 && strings.EqualFold(item.Fields[0], name) {
				return i
			}
		}
		return -1
	}
	for _, childIdx := range allItems[parentIdx].Children {
		if childIdx < len(allItems) && len(allItems[childIdx].Fields) > 0 {
			if strings.EqualFold(allItems[childIdx].Fields[0], name) {
				return childIdx
			}
		}
	}
	return -1
}
