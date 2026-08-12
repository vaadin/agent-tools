package lib

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// defaultIgnore lists directory names that are never descended into.
// Mirrors DEFAULT_IGNORE in src/lib/walk.js.
var defaultIgnore = map[string]bool{
	"node_modules":       true,
	"target":             true,
	"build":              true,
	"dist":               true,
	"out":                true,
	".git":               true,
	".idea":              true,
	".vscode":            true,
	".mvn":               true,
	"frontend-generated": true,
}

// Walk returns absolute paths of regular files under root whose extension is in
// exts (compared case-insensitively, including the leading dot). An empty exts
// matches every file. Unreadable directories are skipped rather than failing the
// whole walk. Results are sorted for deterministic output.
//
// Mirrors walk() in src/lib/walk.js.
func Walk(root string, exts []string) []string {
	extSet := make(map[string]bool, len(exts))
	for _, e := range exts {
		extSet[strings.ToLower(e)] = true
	}

	var results []string
	stack := []string{root}
	for len(stack) > 0 {
		dir := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		entries, err := os.ReadDir(dir)
		if err != nil {
			continue // unreadable directory — skip
		}
		for _, entry := range entries {
			full := filepath.Join(dir, entry.Name())
			if entry.IsDir() {
				if !defaultIgnore[entry.Name()] {
					stack = append(stack, full)
				}
			} else if entry.Type().IsRegular() {
				if len(extSet) == 0 || extSet[strings.ToLower(filepath.Ext(entry.Name()))] {
					results = append(results, full)
				}
			}
		}
	}
	sort.Strings(results)
	return results
}
