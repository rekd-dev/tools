package modules

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestDiscoverWorkspaceRoots(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "mono-ws")
	got := DiscoverWorkspaceRoots(root)
	want := map[string]bool{"apps/alpha": true, "apps/beta": true, "packages/lib": true}
	if len(got) != 3 {
		t.Fatalf("got %#v", got)
	}
	for _, r := range got {
		if !want[filepath.ToSlash(r)] {
			t.Fatalf("unexpected root %q", r)
		}
	}
}

func TestWorkspaceKeyCollisionUsesFullPath(t *testing.T) {
	all := []string{"apps/auth", "packages/auth"}
	if WorkspaceKey("apps/auth", all) != "apps/auth" {
		t.Fatal(WorkspaceKey("apps/auth", all))
	}
}
