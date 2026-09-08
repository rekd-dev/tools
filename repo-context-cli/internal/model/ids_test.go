package model

import "testing"

func TestStableIDs(t *testing.T) {
	if FileID("typescript", `src\domain\entity.ts`) != "file:typescript:src/domain/entity.ts" {
		t.Fatal(FileID("typescript", `src\domain\entity.ts`))
	}
	want := "type:csharp:Application/IEntityRepository.cs:IEntityRepository"
	got := TypeID("csharp", "Application/IEntityRepository.cs", "IEntityRepository")
	if got != want {
		t.Fatalf("got %s", got)
	}
}
