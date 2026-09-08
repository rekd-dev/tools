package modules

import (
	"path/filepath"
	"testing"
)

func TestSkipWalkDirNeverSkipsRoot(t *testing.T) {
	root := filepath.Clean(`/tmp/testdata`)
	if SkipWalkDir(root, root, "testdata") {
		t.Fatal("scan root named testdata must be walked")
	}
}

func TestSkipWalkDirSkipsNestedTestdata(t *testing.T) {
	root := filepath.Clean(`/tmp/repo`)
	nested := filepath.Join(root, "pkg", "testdata")
	if !SkipWalkDir(root, nested, "testdata") {
		t.Fatal("nested testdata should be skipped")
	}
}

func TestSkipWalkDirKeepsNpmPackages(t *testing.T) {
	root := filepath.Clean(`/tmp/repo`)
	pkg := filepath.Join(root, "packages")
	if SkipWalkDir(root, pkg, "packages") {
		t.Fatal("npm packages/ should be scanned")
	}
}

func TestSkipWalkDirSkipsRootE2E(t *testing.T) {
	root := filepath.Clean(`/tmp/repo`)
	e2e := filepath.Join(root, "e2e")
	if !SkipWalkDir(root, e2e, "e2e") {
		t.Fatal("root e2e should be skipped")
	}
}
