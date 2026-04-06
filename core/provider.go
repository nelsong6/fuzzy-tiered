package core

import (
	"os"
	"path/filepath"
	"sort"
)

// TreeProvider loads children for a tree node on demand.
// Implementations determine where data comes from (YAML, filesystem, etc.).
type TreeProvider interface {
	// LoadChildren returns child items for the node at the given path.
	// parentPath is the slash-separated breadcrumb (e.g., "D:/repos/my-homepage").
	// Returns items with Depth, Fields, HasChildren set. ParentIdx is set by the caller.
	LoadChildren(parentPath string) []Item
}

// DirProvider loads directory contents from the filesystem.
type DirProvider struct {
	// ExcludeDirs are directory names to skip (e.g., ".git", "node_modules").
	ExcludeDirs map[string]bool
}

// NewDirProvider creates a DirProvider with common exclusions.
func NewDirProvider() *DirProvider {
	return &DirProvider{
		ExcludeDirs: map[string]bool{
			".git":          true,
			"node_modules":  true,
			"$Recycle.Bin":  true,
			"System Volume Information": true,
		},
	}
}

// LoadChildren reads a directory and returns its entries as Items.
func (p *DirProvider) LoadChildren(parentPath string) []Item {
	entries, err := os.ReadDir(parentPath)
	if err != nil {
		return nil
	}

	var dirs, files []Item
	for _, entry := range entries {
		name := entry.Name()
		if p.ExcludeDirs[name] {
			continue
		}

		fullPath := filepath.Join(parentPath, name)
		item := Item{
			Fields:      []string{name},
			HasChildren: entry.IsDir(),
			ParentIdx:   -1, // set by caller
		}
		if !entry.IsDir() {
			item.Fields = append(item.Fields, fullPath)
		}

		if entry.IsDir() {
			dirs = append(dirs, item)
		} else {
			files = append(files, item)
		}
	}

	// Sort: directories first, then files, each alphabetical
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Fields[0] < dirs[j].Fields[0] })
	sort.Slice(files, func(i, j int) bool { return files[i].Fields[0] < files[j].Fields[0] })

	return append(dirs, files...)
}
