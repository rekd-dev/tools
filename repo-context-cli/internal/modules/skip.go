package modules

import (
	"path/filepath"
	"strings"
)

// IsScanNoiseDir is a directory name that extractors should not walk into.
func IsScanNoiseDir(name string) bool {
	switch name {
	case "bin", "obj", "node_modules", "dist", "build", ".git",
		"coverage", "test-results", "vendor", "testdata":
		return true
	default:
		return false
	}
}

// SkipWalkDir reports whether filepath.Walk should skip dirPath.
// The scan root is never skipped (so testdata fixtures can be analyzed on their own).
func SkipWalkDir(repoRoot, dirPath, name string) bool {
	if filepath.Clean(dirPath) == filepath.Clean(repoRoot) {
		return false
	}
	if IsScanNoiseDir(name) {
		return true
	}
	rel, err := filepath.Rel(repoRoot, dirPath)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	if strings.Contains(rel, "/") {
		return false
	}
	switch name {
	case "e2e", "scripts", "tmp":
		return true
	default:
		return false
	}
}
