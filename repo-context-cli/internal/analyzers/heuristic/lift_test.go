package heuristic_test

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"repo-context-cli/internal/analyzers/heuristic"
	"repo-context-cli/internal/arch"
	"repo-context-cli/internal/fitness"
	"repo-context-cli/internal/model"
)

func TestCsharpFixtureResolvesInterfaceAndCtorInjection(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata", "csharp-ca")
	g := arch.BuildFromRepo(root, fitness.DefaultRules())
	inv := heuristic.Inventory{
		RepoRoot: root,
		Modules:  g.Modules,
		Deps:     g.Deps,
		Types: []heuristic.Type{
			{Name: "IEntityRepository", Kind: "interface", File: "Application/IEntityRepository.cs", Namespace: "Fixture.Application"},
			{Name: "Repo", Kind: "class", File: "Infrastructure/Repo.cs", Implements: []string{"IEntityRepository"}},
			{Name: "EntitiesController", Kind: "class", File: "Api/EntitiesController.cs"},
			{Name: "CreateEntity", Kind: "class", File: "Application/CreateEntity.cs"},
		},
		DI: []heuristic.DI{{Interface: "IEntityRepository", Implementation: "Repo", Lifetime: "scoped", File: "Composition/Registrar.cs"}},
	}
	facts := heuristic.Lift(inv)
	var impl, injects, binds bool
	for _, e := range facts.Edges {
		if e.Kind == model.RelImplements && strings.Contains(e.FromID, "Repo") {
			impl = true
		}
		if e.Kind == model.RelInjects {
			injects = true
		}
		if e.Kind == model.RelBinds {
			binds = true
		}
	}
	if !impl {
		t.Fatal("expected implements Repo -> IEntityRepository")
	}
	if !injects {
		t.Fatal("expected constructor injects")
	}
	if !binds {
		t.Fatal("expected DI binds")
	}
}
