package store

import (
	"os"
	"path/filepath"
	"testing"

	"repo-context-cli/internal/model"
)

func TestReplaceAndLoadGraph(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "inv.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS metadata (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		t.Fatal(err)
	}
	g := model.Graph{
		Nodes: []model.Node{{ID: "type:csharp:a.cs:IRepo", Kind: model.KindType, Name: "IRepo", Abstract: true}},
		Edges: []model.Edge{{ID: "e1", FromID: "a", ToID: "b", Kind: model.RelImplements, Source: "heuristic", Confidence: 0.5, Analyzer: "heuristic"}},
		Coverage: []model.Coverage{{Analyzer: "heuristic", Status: "ran"}},
	}
	if err := ReplaceGraph(db, g); err != nil {
		t.Fatal(err)
	}
	got, err := LoadGraph(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Nodes) != 1 || len(got.Edges) != 1 || len(got.Coverage) != 1 {
		t.Fatalf("%#v", got)
	}
	_ = os.Remove(dbPath)
}
