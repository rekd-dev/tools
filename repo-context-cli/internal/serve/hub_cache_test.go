package serve

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"repo-context-cli/internal/model"
	"repo-context-cli/internal/modules"
)

func TestDBStampMatchesSizeAndMtime(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "inventory.db")
	if err := os.WriteFile(p, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := stampFile(p)
	if err != nil {
		t.Fatal(err)
	}
	b, err := stampFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !a.matches(b) {
		t.Fatal("unchanged file should match")
	}
	time.Sleep(15 * time.Millisecond)
	if err := os.WriteFile(p, []byte("two-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := stampFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if a.matches(c) {
		t.Fatal("rewritten file should be a new stamp")
	}
}

func TestGraphHubDropsCacheWhenDBChanges(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "inventory.db")
	if err := os.WriteFile(p, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := stampFile(p)
	if err != nil {
		t.Fatal(err)
	}
	h := &graphHub{
		dbPath:      p,
		cached:      &modules.Graph{Modules: []modules.Module{{ID: "old"}}},
		cachedFacts: &model.Graph{Nodes: []model.Node{{ID: "old"}}},
		dbStamp:     st,
	}
	h.dropStaleCache()
	if h.cached == nil || h.cachedFacts == nil {
		t.Fatal("unchanged db should keep cache")
	}
	time.Sleep(15 * time.Millisecond)
	if err := os.WriteFile(p, []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.dropStaleCache()
	if h.cached != nil || h.cachedFacts != nil {
		t.Fatal("db change should drop module and fact caches")
	}
}
