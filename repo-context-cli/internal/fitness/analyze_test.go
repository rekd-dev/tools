package fitness

import (
	"encoding/json"
	"testing"

	"repo-context-cli/internal/modules"
)

func TestLayerViolationDomainToInfra(t *testing.T) {
	mods := []modules.Module{
		{ID: "src/domain/entity.ts", Path: "src/domain/entity.ts", Language: "typescript"},
		{ID: "src/infrastructure/db.ts", Path: "src/infrastructure/db.ts", Language: "typescript"},
	}
	deps := []modules.Dep{
		{FromID: "src/domain/entity.ts", ToID: "src/infrastructure/db.ts", Kind: "direct"},
	}
	r := Analyze("test", "/tmp/test", "", mods, deps, DefaultRules(), "default")
	found := false
	for _, f := range r.Findings {
		if f.Kind == "layer_violation" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected layer_violation, got %#v", r.Findings)
	}
}

func TestDomainPurity(t *testing.T) {
	mods := []modules.Module{
		{
			ID: "src/domain/entity.ts", Path: "src/domain/entity.ts", Language: "typescript",
			ExtImports: []string{"better-sqlite3"},
		},
	}
	r := Analyze("test", "/tmp/test", "", mods, nil, DefaultRules(), "default")
	found := false
	for _, f := range r.Findings {
		if f.Kind == "domain_purity" && f.Severity == SeverityError {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected domain_purity error, got %#v", r.Findings)
	}
}

func TestAllowedApplicationToDomain(t *testing.T) {
	mods := []modules.Module{
		{ID: "src/domain/entity.ts", Path: "src/domain/entity.ts", Language: "typescript"},
		{ID: "src/application/uc.ts", Path: "src/application/uc.ts", Language: "typescript"},
	}
	deps := []modules.Dep{
		{FromID: "src/application/uc.ts", ToID: "src/domain/entity.ts", Kind: "direct"},
	}
	r := Analyze("test", "/tmp/test", "", mods, deps, DefaultRules(), "default")
	for _, f := range r.Findings {
		if f.Kind == "layer_violation" {
			t.Fatalf("unexpected layer violation: %s", f.Message)
		}
	}
}

func TestCleanReportJSONHasEmptyArrays(t *testing.T) {
	r := Analyze("test", "/tmp/test", "", nil, nil, DefaultRules(), "default")
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"findings", "modules"} {
		s := string(decoded[key])
		if s == "null" {
			t.Fatalf("%s marshaled as null", key)
		}
	}
}
