package fitness

import (
	"encoding/json"
	"testing"

	"repo-context-cli/internal/modules"
)

func TestAssignLayersClassifiesCompositionAndHTTPPaths(t *testing.T) {
	mods := []modules.Module{
		{ID: "container", Path: "apps/schedule-api/src/composition/container.ts"},
		{ID: "route", Path: "apps/schedule-api/src/http/routes/shifts.ts"},
		{ID: "use-case", Path: "apps/schedule-api/src/application/use-cases/update-shift.ts"},
	}

	got := AssignLayers(mods, DefaultRules())

	if got[0].Layer != "composition" {
		t.Fatalf("composition container layer = %q, want composition", got[0].Layer)
	}
	if got[1].Layer != "interface" {
		t.Fatalf("HTTP route layer = %q, want interface", got[1].Layer)
	}
	if got[2].Layer != "application" {
		t.Fatalf("use case layer = %q, want application", got[2].Layer)
	}
}

func TestAssignLayersIgnoresTestAndE2EPaths(t *testing.T) {
	mods := []modules.Module{
		{ID: "unit", Path: "apps/api/src/application/use-cases/create-user.test.ts", Layer: "application"},
		{ID: "spec", Path: "apps/api/src/http/routes/users.spec.ts"},
		{ID: "tests", Path: "apps/api/tests/application/create-user.ts"},
		{ID: "e2e", Path: "apps/web/e2e/login.ts"},
		{ID: "playwright", Path: "apps/web/playwright/auth.setup.ts"},
	}

	got := AssignLayers(mods, DefaultRules())

	for _, m := range got {
		if m.Layer != "" {
			t.Errorf("%s layer = %q, want ignored", m.Path, m.Layer)
		}
	}

	report := Analyze("test", "/tmp/test", "", mods, nil, DefaultRules(), "default")
	if report.Summary.Unlayered != 0 {
		t.Fatalf("ignored paths counted as %d unlayered modules", report.Summary.Unlayered)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("ignored paths produced findings: %#v", report.Findings)
	}
}

func TestAnalyzeKeepsApplicationToCompositionViolation(t *testing.T) {
	mods := []modules.Module{
		{ID: "use-case", Path: "src/application/use-cases/create-user.ts"},
		{ID: "container", Path: "src/composition/container.ts"},
	}
	deps := []modules.Dep{
		{FromID: "use-case", ToID: "container", Kind: "direct"},
	}

	r := Analyze("test", "/tmp/test", "", mods, deps, DefaultRules(), "default")

	if len(r.Findings) != 1 || r.Findings[0].Kind != "layer_violation" {
		t.Fatalf("expected application-to-composition violation, got %#v", r.Findings)
	}
}

func TestAnalyzeExcludesIgnoredTestsFromCyclesAndDependencies(t *testing.T) {
	mods := []modules.Module{
		{ID: "use-case", Path: "src/application/use-cases/create-user.ts"},
		{ID: "test", Path: "src/application/use-cases/create-user.test.ts"},
	}
	deps := []modules.Dep{
		{FromID: "use-case", ToID: "test", Kind: "direct"},
		{FromID: "test", ToID: "use-case", Kind: "direct"},
	}

	r := Analyze("test", "/tmp/test", "", mods, deps, DefaultRules(), "default")

	if len(r.Cycles) != 0 {
		t.Fatalf("ignored test produced cycles: %#v", r.Cycles)
	}
	if len(r.Findings) != 0 {
		t.Fatalf("ignored test dependency produced findings: %#v", r.Findings)
	}
}

func TestAnalyzeGroupsCompositionRootViolationsBySourceLayer(t *testing.T) {
	mods := []modules.Module{
		{ID: "route-a", Path: "src/http/routes/a.ts"},
		{ID: "route-b", Path: "src/http/routes/b.ts"},
		{ID: "use-case", Path: "src/application/use-cases/create-user.ts"},
		{ID: "container", Path: "src/composition/container.ts"},
	}
	deps := []modules.Dep{
		{FromID: "route-a", ToID: "container", Kind: "direct"},
		{FromID: "route-b", ToID: "container", Kind: "direct"},
		{FromID: "use-case", ToID: "container", Kind: "direct"},
	}

	r := Analyze("test", "/tmp/test", "", mods, deps, DefaultRules(), "default")

	if len(r.Findings) != 2 {
		t.Fatalf("findings = %#v, want one interface and one application group", r.Findings)
	}
	if r.Summary.LayerHits != 3 {
		t.Fatalf("layer violations = %d, want all 3 underlying edges", r.Summary.LayerHits)
	}
	var interfaceFinding Finding
	for _, f := range r.Findings {
		if f.Layer == "interface" {
			interfaceFinding = f
		}
	}
	if interfaceFinding.RelatedCount != 2 {
		t.Fatalf("interface related count = %d, want 2", interfaceFinding.RelatedCount)
	}
}

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

func TestTypeOnlyInterfaceToCompositionIsInfo(t *testing.T) {
	mods := []modules.Module{
		{ID: "route", Path: "src/http/routes.ts"},
		{ID: "container", Path: "src/composition/container.ts"},
	}
	deps := []modules.Dep{
		{FromID: "route", ToID: "container", Kind: "direct", TypeOnly: true},
	}

	r := Analyze("test", "/tmp/test", "", mods, deps, DefaultRules(), "default")

	if len(r.Findings) != 1 {
		t.Fatalf("findings = %#v, want 1 type-only layer finding", r.Findings)
	}
	f := r.Findings[0]
	if f.Kind != "layer_violation_type" {
		t.Fatalf("kind = %q, want layer_violation_type", f.Kind)
	}
	if f.Severity != SeverityInfo {
		t.Fatalf("severity = %q, want info", f.Severity)
	}
	if r.Summary.Errors != 0 {
		t.Fatalf("errors = %d, want 0 (type-only is not an error)", r.Summary.Errors)
	}
}

func TestStoredKindTypeIsInfoWithoutTypeOnlyFlag(t *testing.T) {
	mods := []modules.Module{
		{ID: "route", Path: "src/http/routes.ts"},
		{ID: "container", Path: "src/composition/container.ts"},
	}
	deps := []modules.Dep{
		{FromID: "route", ToID: "container", Kind: "type"},
	}

	r := Analyze("test", "/tmp/test", "", mods, deps, DefaultRules(), "default")
	if len(r.Findings) != 1 || r.Findings[0].Kind != "layer_violation_type" || r.Findings[0].Severity != SeverityInfo {
		t.Fatalf("findings = %#v", r.Findings)
	}
}

func TestValueApplicationToInfrastructureIsError(t *testing.T) {
	mods := []modules.Module{
		{ID: "uc", Path: "src/application/use-cases/create-user.ts"},
		{ID: "db", Path: "src/infrastructure/db.ts"},
	}
	deps := []modules.Dep{
		{FromID: "uc", ToID: "db", Kind: "direct"},
	}

	r := Analyze("test", "/tmp/test", "", mods, deps, DefaultRules(), "default")

	found := false
	for _, f := range r.Findings {
		if f.Kind == "layer_violation" && f.Severity == SeverityError {
			found = true
		}
		if f.Kind == "layer_violation_type" {
			t.Fatalf("value import treated as type-only: %#v", f)
		}
	}
	if !found {
		t.Fatalf("expected value application→infrastructure error, got %#v", r.Findings)
	}
}

func TestTypeOnlyDomainCycleIsNotError(t *testing.T) {
	mods := []modules.Module{
		{ID: "a", Path: "src/domain/a.ts"},
		{ID: "b", Path: "src/domain/b.ts"},
	}
	deps := []modules.Dep{
		{FromID: "a", ToID: "b", Kind: "direct", TypeOnly: true},
		{FromID: "b", ToID: "a", Kind: "direct", TypeOnly: true},
	}

	r := Analyze("test", "/tmp/test", "", mods, deps, DefaultRules(), "default")

	if len(r.Cycles) != 0 {
		t.Fatalf("type-only cycle emitted: %#v", r.Cycles)
	}
	for _, f := range r.Findings {
		if f.Kind == "cycle" {
			t.Fatalf("type-only domain cycle produced cycle finding: %#v", f)
		}
	}
}

func TestValueDomainCycleIsError(t *testing.T) {
	mods := []modules.Module{
		{ID: "a", Path: "src/domain/a.ts"},
		{ID: "b", Path: "src/domain/b.ts"},
	}
	deps := []modules.Dep{
		{FromID: "a", ToID: "b", Kind: "direct"},
		{FromID: "b", ToID: "a", Kind: "direct"},
	}

	r := Analyze("test", "/tmp/test", "", mods, deps, DefaultRules(), "default")

	if len(r.Cycles) == 0 {
		t.Fatalf("expected value domain cycle, findings=%#v", r.Findings)
	}
	found := false
	for _, f := range r.Findings {
		if f.Kind == "cycle" && f.Severity == SeverityError {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected cycle error, got %#v", r.Findings)
	}
}

func TestTypeOnlyAndValueCompositionGroupsNotMixed(t *testing.T) {
	mods := []modules.Module{
		{ID: "route-type", Path: "src/http/routes/type.ts"},
		{ID: "route-value", Path: "src/http/routes/value.ts"},
		{ID: "container", Path: "src/composition/container.ts"},
	}
	deps := []modules.Dep{
		{FromID: "route-type", ToID: "container", Kind: "direct", TypeOnly: true},
		{FromID: "route-value", ToID: "container", Kind: "direct"},
	}

	r := Analyze("test", "/tmp/test", "", mods, deps, DefaultRules(), "default")

	var typeFinding, valueFinding *Finding
	for i := range r.Findings {
		f := &r.Findings[i]
		switch f.Kind {
		case "layer_violation_type":
			typeFinding = f
		case "layer_violation":
			valueFinding = f
		}
	}
	if typeFinding == nil || valueFinding == nil {
		t.Fatalf("want separate type-only info and value error groups, got %#v", r.Findings)
	}
	if typeFinding.Severity != SeverityInfo {
		t.Fatalf("type-only group severity = %q", typeFinding.Severity)
	}
	if valueFinding.Severity != SeverityError {
		t.Fatalf("value group severity = %q", valueFinding.Severity)
	}
	if typeFinding.RelatedCount != 0 || valueFinding.RelatedCount != 0 {
		t.Fatalf("groups were mixed: type=%#v value=%#v", typeFinding, valueFinding)
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
