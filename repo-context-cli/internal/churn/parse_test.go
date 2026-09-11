package churn

import (
	"testing"
)

func TestParseLogCountsCommitsPerRoot(t *testing.T) {
	raw := "" +
		"COMMIT\tAda\t2026-01-02\n" +
		"apps/schedule-api/src/http/routes.ts\n" +
		"apps/schedule-web/src/App.tsx\n" +
		"\n" +
		"COMMIT\tAda\t2026-02-01\n" +
		"apps/schedule-api/src/domain/user.ts\n" +
		"\n" +
		"COMMIT\tBob\t2026-03-01\n" +
		"apps/reporting-web/src/App.tsx\n"

	got := ParseLog(raw, []Root{
		{ID: "schedule-api", FileRoot: "apps/schedule-api"},
		{ID: "schedule-web", FileRoot: "apps/schedule-web"},
		{ID: "reporting-web", FileRoot: "apps/reporting-web"},
	})
	if got["schedule-api"].Commits != 2 {
		t.Fatalf("schedule-api commits=%d", got["schedule-api"].Commits)
	}
	if got["schedule-api"].Authors != 1 || got["schedule-api"].Last != "2026-02-01" {
		t.Fatalf("%#v", got["schedule-api"])
	}
	if got["schedule-web"].Commits != 1 {
		t.Fatalf("schedule-web commits=%d", got["schedule-web"].Commits)
	}
	if got["reporting-web"].Authors != 1 {
		t.Fatalf("reporting-web authors=%d", got["reporting-web"].Authors)
	}
}

func TestParseLogDoesNotSubstringMatch(t *testing.T) {
	raw := "COMMIT\tAda\t2026-01-02\napps/schedule-api/src/a.ts\n"
	got := ParseLog(raw, []Root{{ID: "auth", FileRoot: "packages/auth"}})
	if _, ok := got["auth"]; ok {
		t.Fatalf("auth should not match schedule-api: %#v", got)
	}
}
