package ts

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestExtractImportSpecsTypeOnly(t *testing.T) {
	src := `
import type { AppContainer } from './container'
import type WiredApplication from './wired'
export type { X } from './container'
import { AppContainer } from './container-value'
import './side-effect'
const x = require('./req')
`
	got := extractImportSpecs(src)
	want := []importSpec{
		{Spec: "./container", TypeOnly: true},
		{Spec: "./wired", TypeOnly: true},
		{Spec: "./container", TypeOnly: true},
		{Spec: "./container-value", TypeOnly: false},
		{Spec: "./side-effect"},
		{Spec: "./req"},
	}
	if len(got) != len(want) {
		t.Fatalf("specs = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("spec[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestScanTypeOnlyImportFromFixture(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata", "ts-ca")
	res := Scan(root, nil, nil, nil)

	from := "src/http/routes.ts"
	toPorts := "src/application/ports.ts"
	toCreate := "src/application/create.ts"
	var ports, create bool
	for _, d := range res.Deps {
		if d.FromID != from {
			continue
		}
		if d.ToID == toPorts {
			ports = true
			if !d.TypeOnly {
				t.Fatalf("import type ports dep TypeOnly=false: %#v", d)
			}
		}
		if d.ToID == toCreate {
			create = true
			if d.TypeOnly {
				t.Fatalf("value create import marked type-only: %#v", d)
			}
		}
	}
	if !ports || !create {
		t.Fatalf("expected both ports (type) and create (value) deps from %s, got %#v", from, res.Deps)
	}
}

func TestScanValueImportWinsOverTypeOnly(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(name, body string) {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("a.ts", `
import type { B } from './b'
import { b } from './b'
`)
	mustWrite("b.ts", `
export type B = number
export const b = 1
`)

	res := Scan(dir, nil, nil, nil)
	var found bool
	for _, d := range res.Deps {
		if d.FromID == "a.ts" && d.ToID == "b.ts" {
			found = true
			if d.TypeOnly {
				t.Fatalf("value+type import should keep TypeOnly=false, got %#v", d)
			}
		}
	}
	if !found {
		t.Fatalf("missing a.ts → b.ts dep: %#v", res.Deps)
	}
}

func TestScanTypeOnlyOnlyStaysTypeOnly(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.ts"), []byte("import type { B } from './b'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.ts"), []byte("export type B = number\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := Scan(dir, nil, nil, nil)
	for _, d := range res.Deps {
		if d.FromID == "a.ts" && d.ToID == "b.ts" {
			if !d.TypeOnly {
				t.Fatalf("type-only import TypeOnly=false: %#v", d)
			}
			return
		}
	}
	t.Fatalf("missing a.ts → b.ts dep: %#v", res.Deps)
}
