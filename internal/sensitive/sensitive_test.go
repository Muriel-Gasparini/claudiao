package sensitive

import (
	"slices"
	"testing"
)

func TestMatchPath(t *testing.T) {
	cases := []struct {
		path string
		want Area
	}{
		{"internal/auth/login.go", AreaAuth},
		{"src/session_store.ts", AreaAuth},
		{"pkg/crypto/cipher.go", AreaCrypto},
		{".env.production", AreaSecrets},
		{"config/credentials.yaml", AreaSecrets},
		{"db/migrations/0001_create_users.sql", AreaMigrations},
		{"go.mod", AreaDeps},
		{"package.json", AreaDeps},
		{"api/handlers/user.go", AreaEndpoints},
		{"internal/permissions/rbac.go", AreaAuthz},
	}
	for _, c := range cases {
		got := MatchPath(c.path)
		if !slices.Contains(got, c.want) {
			t.Errorf("MatchPath(%q) = %v, want to contain %v", c.path, got, c.want)
		}
	}
}

func TestMatchPathCleanFiles(t *testing.T) {
	for _, p := range []string{"README.md", "internal/tui/styles.go", "docs/guide.md", "main.go"} {
		if got := MatchPath(p); len(got) != 0 {
			t.Errorf("MatchPath(%q) = %v, want none", p, got)
		}
	}
}

func TestMatchDiffAddedLinesOnly(t *testing.T) {
	diff := `diff --git a/server.go b/server.go
--- a/server.go
+++ b/server.go
@@ -1,3 +1,4 @@
 package main
-const old = "SELECT name FROM users WHERE id = $1"
+const q = "SELECT name FROM users WHERE id = " + userInput
`
	areas := MatchDiff(diff)
	if !slices.Contains(areas, AreaDatabase) {
		t.Errorf("expected database area, got %v", areas)
	}
}

func TestMatchDiffRemovalDoesNotTrigger(t *testing.T) {
	diff := `diff --git a/clean.go b/clean.go
--- a/clean.go
+++ b/clean.go
@@ -1,2 +1,1 @@
-token := jwt.Sign(claims)
 fmt.Println("hello")
`
	if areas := MatchDiff(diff); len(areas) != 0 {
		t.Errorf("removed lines must not trigger, got %v", areas)
	}
}

func TestJoin(t *testing.T) {
	if got := Join([]Area{AreaAuth, AreaCrypto}); got != "auth/sessions, crypto" {
		t.Errorf("Join = %q", got)
	}
}
