package core

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// YAMLEntry represents a single node in the YAML tree.
// Children can be either inline entries or a file path string.
type YAMLEntry struct {
	Name        string      `yaml:"name"`
	Description string      `yaml:"description,omitempty"`
	URL         string      `yaml:"url,omitempty"`
	Children    interface{} `yaml:"children,omitempty"` // []YAMLEntry or string (file path)
}

// LoadYAMLFromString parses YAML content directly without file I/O.
// File-reference children (children: "path/to/file.yaml") are not supported
// and will return an error.
func LoadYAMLFromString(content string) ([]Item, error) {
	var entries []YAMLEntry
	if err := yaml.Unmarshal([]byte(content), &entries); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}
	var items []Item
	if err := flattenYAML(entries, "", 0, -1, "", &items); err != nil {
		return nil, err
	}
	return items, nil
}

// LoadYAML reads a YAML file and recursively resolves file pointers,
// returning a flat list of Items with depth, parent, and children indices.
func LoadYAML(path string) ([]Item, error) {
	entries, err := readYAMLFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading %s: %w", path, err)
	}

	baseDir := filepath.Dir(path)
	var items []Item
	if err := flattenYAML(entries, baseDir, 0, -1, "", &items); err != nil {
		return nil, err
	}
	return items, nil
}

func readYAMLFile(path string) ([]YAMLEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var entries []YAMLEntry
	if err := yaml.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return entries, nil
}

func flattenYAML(entries []YAMLEntry, baseDir string, depth int, parentIdx int, parentPath string, items *[]Item) error {
	for _, e := range entries {
		fields := []string{e.Name}
		if e.Description != "" {
			fields = append(fields, e.Description)
		}

		myIdx := len(*items)
		hasChildren := e.Children != nil

		path := e.Name
		if parentPath != "" {
			path = parentPath + " › " + e.Name
		}

		*items = append(*items, Item{
			Fields:      fields,
			Depth:       depth,
			ParentIdx:   parentIdx,
			HasChildren: hasChildren,
			Path:        path,
			URL:         e.URL,
		})

		// Register this item as a child of its parent
		if parentIdx >= 0 {
			(*items)[parentIdx].Children = append((*items)[parentIdx].Children, myIdx)
		}

		if !hasChildren {
			continue
		}

		switch children := e.Children.(type) {
		case string:
			childPath := children
			if !filepath.IsAbs(childPath) {
				childPath = filepath.Join(baseDir, childPath)
			}
			childEntries, err := readYAMLFile(childPath)
			if err != nil {
				return fmt.Errorf("resolving children for %q: %w", e.Name, err)
			}
			childBaseDir := filepath.Dir(childPath)
			if err := flattenYAML(childEntries, childBaseDir, depth+1, myIdx, path, items); err != nil {
				return err
			}

		case []interface{}:
			inlineEntries, err := parseInlineYAMLChildren(children)
			if err != nil {
				return fmt.Errorf("parsing inline children for %q: %w", e.Name, err)
			}
			if err := flattenYAML(inlineEntries, baseDir, depth+1, myIdx, path, items); err != nil {
				return err
			}
		}
	}
	return nil
}

func parseInlineYAMLChildren(raw []interface{}) ([]YAMLEntry, error) {
	data, err := yaml.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var entries []YAMLEntry
	if err := yaml.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}
