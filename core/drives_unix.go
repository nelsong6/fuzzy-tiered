//go:build !windows

package core

// ListDriveRoots returns the filesystem root as a single tree item on Unix.
func ListDriveRoots() []Item {
	return []Item{
		{
			Fields:      []string{"/"},
			Depth:       0,
			HasChildren: true,
			ParentIdx:   -1,
		},
	}
}
