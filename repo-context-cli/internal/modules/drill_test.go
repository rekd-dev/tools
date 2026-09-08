package modules

import "testing"

func TestComputeDrillPathKeepsFilenameDots(t *testing.T) {
	got := ComputeDrillPath("typescript", "", "arch-view/vite.config.ts", nil)
	if got != "arch-view/vite.config" {
		t.Fatalf("got %q", got)
	}
}

func TestComputeDrillPathStripsSrc(t *testing.T) {
	got := ComputeDrillPath("typescript", "", "apps/api/src/domain/entity.ts", nil)
	if got != "domain/entity" {
		t.Fatalf("got %q", got)
	}
}

func TestComputeDrillPathWorkspaceKeepsPackage(t *testing.T) {
	ws := []string{"apps/schedule-api", "apps/safelog-api", "packages/auth"}
	got := ComputeDrillPath("typescript", "", "apps/schedule-api/src/domain/entity.ts", ws)
	if got != "schedule-api/domain/entity" {
		t.Fatalf("got %q", got)
	}
	got = ComputeDrillPath("typescript", "", "apps/safelog-api/src/application/uc.ts", ws)
	if got != "safelog-api/application/uc" {
		t.Fatalf("got %q", got)
	}
	got = ComputeDrillPath("typescript", "", "packages/auth/src/index.ts", ws)
	if got != "auth/index" {
		t.Fatalf("got %q", got)
	}
}

func TestComputeDrillPathCSharpNamespace(t *testing.T) {
	got := ComputeDrillPath("csharp", "Fixture.Application", "Application/CreateEntity.cs", nil)
	if got != "Fixture/Application" {
		t.Fatalf("got %q", got)
	}
}

func TestSplitDrillPathKeepsFilenameDots(t *testing.T) {
	got := SplitDrillPath("arch-view/vite.config")
	if len(got) != 2 || got[1] != "vite.config" {
		t.Fatalf("got %#v", got)
	}
}
