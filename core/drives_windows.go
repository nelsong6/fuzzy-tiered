//go:build windows

package core

import "os"

// ListDriveRoots returns available Windows drive letters as root tree items.
func ListDriveRoots() []Item {
	var items []Item
	for _, drive := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		path := string(drive) + ":\\"
		if _, err := os.Stat(path); err == nil {
			items = append(items, Item{
				Fields:      []string{string(drive) + ":"},
				Depth:       0,
				HasChildren: true,
				ParentIdx:   -1,
			})
		}
	}
	return items
}
