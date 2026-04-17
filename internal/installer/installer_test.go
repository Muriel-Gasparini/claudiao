package installer

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func testAssets() fstest.MapFS {
	return fstest.MapFS{
		"CLAUDE.md":           {Data: []byte("# Claude\n")},
		"rules/testing.md":    {Data: []byte("testing rules\n")},
		"rules/git.md":        {Data: []byte("git rules\n")},
		"commands/sdd-new.md": {Data: []byte("command\n")},
		"agents/product.md":   {Data: []byte("agent\n")},
		"rules/.gitkeep":      {Data: []byte("")},
		"README.md":           {Data: []byte("readme\n")},
	}
}

func TestCoreModuleInstallsRootFiles(t *testing.T) {
	dir := t.TempDir()
	plan, err := Preview(Request{
		ClaudePath: dir,
		Assets:     testAssets(),
		Modules:    []string{"core"},
		Mode:       ModeCopy,
	})
	if err != nil {
		t.Fatal(err)
	}
	var foundClaude bool
	for _, a := range plan.Actions {
		if a.Source == "CLAUDE.md" {
			foundClaude = true
		}
	}
	if !foundClaude {
		t.Error("expected CLAUDE.md in core module")
	}
	if len(plan.Actions) != 1 {
		t.Errorf("expected only CLAUDE.md in core, got %d", len(plan.Actions))
	}
}

func TestReadmeAtRootNotInstalledEvenWithCore(t *testing.T) {
	dir := t.TempDir()
	plan, _ := Preview(Request{
		ClaudePath: dir,
		Assets:     testAssets(),
		Modules:    []string{"core"},
		Mode:       ModeCopy,
	})
	for _, a := range plan.Actions {
		if filepath.Base(a.Source) == "README.md" {
			t.Error("README.md should be filtered out")
		}
	}
}

func TestPreviewAllCreate(t *testing.T) {
	dir := t.TempDir()
	plan, err := Preview(Request{
		ClaudePath: dir,
		Assets:     testAssets(),
		Modules:    []string{"rules", "commands", "agents", "core"},
		Mode:       ModeCopy,
	})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(plan.Actions) != 5 {
		t.Fatalf("expected 5 actions, got %d", len(plan.Actions))
	}
	for _, a := range plan.Actions {
		if a.Kind != KindCreate {
			t.Errorf("expected create for %s, got %s", a.Source, a.Kind)
		}
	}
}

func TestPreviewFilterByModule(t *testing.T) {
	dir := t.TempDir()
	plan, err := Preview(Request{
		ClaudePath: dir,
		Assets:     testAssets(),
		Modules:    []string{"rules"},
		Mode:       ModeCopy,
	})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	for _, a := range plan.Actions {
		if filepath.ToSlash(a.Source)[:5] != "rules" {
			t.Errorf("unexpected module in %s", a.Source)
		}
	}
	if len(plan.Actions) != 2 {
		t.Fatalf("expected 2 rules actions, got %d", len(plan.Actions))
	}
}

func TestPreviewSkipsGitkeepAndReadme(t *testing.T) {
	dir := t.TempDir()
	plan, err := Preview(Request{
		ClaudePath: dir,
		Assets:     testAssets(),
		Modules:    []string{"rules"},
		Mode:       ModeCopy,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range plan.Actions {
		base := filepath.Base(a.Source)
		if base == ".gitkeep" || base == "README.md" {
			t.Errorf("should not install %s", a.Source)
		}
	}
}

func TestPreviewDetectsSameAndDiffer(t *testing.T) {
	dir := t.TempDir()
	assets := testAssets()

	if err := os.MkdirAll(filepath.Join(dir, "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rules", "testing.md"), assets["rules/testing.md"].Data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rules", "git.md"), []byte("LOCAL VERSION\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	plan, err := Preview(Request{
		ClaudePath: dir,
		Assets:     assets,
		Modules:    []string{"rules"},
		Mode:       ModeCopy,
	})
	if err != nil {
		t.Fatal(err)
	}

	kinds := map[string]Kind{}
	for _, a := range plan.Actions {
		rel, _ := filepath.Rel(dir, a.Target)
		kinds[filepath.ToSlash(rel)] = a.Kind
	}
	if kinds["rules/testing.md"] != KindSame {
		t.Errorf("expected same for testing.md, got %s", kinds["rules/testing.md"])
	}
	if kinds["rules/git.md"] != KindDiffer {
		t.Errorf("expected differ for git.md, got %s", kinds["rules/git.md"])
	}
}

func TestApplyCopyWritesFiles(t *testing.T) {
	dir := t.TempDir()
	assets := testAssets()
	req := Request{
		ClaudePath: dir,
		Assets:     assets,
		Modules:    []string{"rules"},
		Mode:       ModeCopy,
	}
	plan, err := Preview(req)
	if err != nil {
		t.Fatal(err)
	}
	var progressCalls int
	err = Apply(plan, req, func(done, total int, current string) {
		progressCalls++
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if progressCalls != 2 {
		t.Errorf("expected 2 progress calls, got %d", progressCalls)
	}
	data, err := os.ReadFile(filepath.Join(dir, "rules", "testing.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "testing rules\n" {
		t.Errorf("unexpected content: %q", string(data))
	}
}

func TestApplySymlinkRequiresAssetsDir(t *testing.T) {
	_, err := Preview(Request{
		ClaudePath: t.TempDir(),
		Assets:     testAssets(),
		Mode:       ModeSymlink,
	})
	if err == nil {
		t.Fatal("expected error when AssetsDir empty for symlink mode")
	}
}

func TestApplySymlinkCreatesLinks(t *testing.T) {
	claudePath := t.TempDir()
	assetsDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(assetsDir, "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "rules", "testing.md"), []byte("from disk\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	assets := fstest.MapFS{
		"rules/testing.md": {Data: []byte("from disk\n")},
	}

	req := Request{
		ClaudePath: claudePath,
		Assets:     assets,
		AssetsDir:  assetsDir,
		Modules:    []string{"rules"},
		Mode:       ModeSymlink,
	}
	plan, err := Preview(req)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan, req, nil); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(filepath.Join(claudePath, "rules", "testing.md"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("expected symlink")
	}
	dest, err := os.Readlink(filepath.Join(claudePath, "rules", "testing.md"))
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(assetsDir, "rules", "testing.md")
	if dest != expected {
		t.Errorf("symlink points to %s, expected %s", dest, expected)
	}
}

func TestBackupCopiesTreeWithTimestamp(t *testing.T) {
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "rules", "testing.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	backup, err := Backup(src)
	if err != nil {
		t.Fatal(err)
	}
	if backup == "" {
		t.Fatal("expected backup path")
	}
	data, err := os.ReadFile(filepath.Join(backup, "rules", "testing.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello\n" {
		t.Errorf("backup content mismatch: %q", string(data))
	}
	_ = os.RemoveAll(backup)
}

func TestBackupReturnsEmptyWhenMissing(t *testing.T) {
	path, err := Backup(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatal(err)
	}
	if path != "" {
		t.Errorf("expected empty backup path, got %s", path)
	}
}

func TestKindString(t *testing.T) {
	cases := map[Kind]string{
		KindCreate: "create",
		KindSame:   "same",
		KindDiffer: "differ",
		Kind(99):   "unknown",
	}
	for k, want := range cases {
		if got := k.String(); got != want {
			t.Errorf("Kind(%d).String() = %q, want %q", k, got, want)
		}
	}
}

func TestPreviewRequiresClaudePath(t *testing.T) {
	_, err := Preview(Request{Assets: testAssets()})
	if err == nil {
		t.Fatal("expected error for empty ClaudePath")
	}
}

func TestPreviewRequiresAssets(t *testing.T) {
	_, err := Preview(Request{ClaudePath: t.TempDir()})
	if err == nil {
		t.Fatal("expected error for nil Assets")
	}
}

func TestSymlinkSameWhenAlreadyLinkedCorrectly(t *testing.T) {
	claudePath := t.TempDir()
	assetsDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(assetsDir, "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(assetsDir, "rules", "testing.md")
	if err := os.WriteFile(src, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(claudePath, "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(src, filepath.Join(claudePath, "rules", "testing.md")); err != nil {
		t.Fatal(err)
	}

	assets := fstest.MapFS{"rules/testing.md": {Data: []byte("x\n")}}
	plan, err := Preview(Request{
		ClaudePath: claudePath,
		Assets:     assets,
		AssetsDir:  assetsDir,
		Modules:    []string{"rules"},
		Mode:       ModeSymlink,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(plan.Actions))
	}
	if plan.Actions[0].Kind != KindSame {
		t.Errorf("expected KindSame, got %s", plan.Actions[0].Kind)
	}
}

func TestSymlinkDifferWhenPointingElsewhere(t *testing.T) {
	claudePath := t.TempDir()
	assetsDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(assetsDir, "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "rules", "testing.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	elsewhere := filepath.Join(t.TempDir(), "something.md")
	if err := os.WriteFile(elsewhere, []byte("other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(claudePath, "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, filepath.Join(claudePath, "rules", "testing.md")); err != nil {
		t.Fatal(err)
	}

	assets := fstest.MapFS{"rules/testing.md": {Data: []byte("x\n")}}
	plan, err := Preview(Request{
		ClaudePath: claudePath,
		Assets:     assets,
		AssetsDir:  assetsDir,
		Modules:    []string{"rules"},
		Mode:       ModeSymlink,
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Actions[0].Kind != KindDiffer {
		t.Errorf("expected KindDiffer, got %s", plan.Actions[0].Kind)
	}
}

func TestApplyOverwritesExistingDifferent(t *testing.T) {
	dir := t.TempDir()
	assets := fstest.MapFS{"rules/testing.md": {Data: []byte("new\n")}}
	if err := os.MkdirAll(filepath.Join(dir, "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rules", "testing.md"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	req := Request{
		ClaudePath: dir,
		Assets:     assets,
		Modules:    []string{"rules"},
		Mode:       ModeCopy,
	}
	plan, err := Preview(req)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Actions[0].Kind != KindDiffer {
		t.Fatalf("expected differ, got %s", plan.Actions[0].Kind)
	}
	if err := Apply(plan, req, nil); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "rules", "testing.md"))
	if string(data) != "new\n" {
		t.Errorf("expected overwrite, content is %q", data)
	}
}

func TestApplySkipsIdenticalFiles(t *testing.T) {
	dir := t.TempDir()
	assets := fstest.MapFS{"rules/testing.md": {Data: []byte("same\n")}}
	if err := os.MkdirAll(filepath.Join(dir, "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rules", "testing.md"), []byte("same\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	req := Request{
		ClaudePath: dir,
		Assets:     assets,
		Modules:    []string{"rules"},
		Mode:       ModeCopy,
	}
	plan, _ := Preview(req)
	var calls int
	_ = Apply(plan, req, func(done, total int, current string) { calls++ })
	if calls != 0 {
		t.Errorf("expected no progress calls for same files, got %d", calls)
	}
}

func TestApplyNilPlan(t *testing.T) {
	err := Apply(nil, Request{}, nil)
	if err == nil {
		t.Fatal("expected error for nil plan")
	}
}

func TestBackupPreservesSymlinks(t *testing.T) {
	src := t.TempDir()
	target := filepath.Join(t.TempDir(), "real.md")
	if err := os.WriteFile(target, []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(src, "link.md")); err != nil {
		t.Fatal(err)
	}

	backup, err := Backup(src)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(filepath.Join(backup, "link.md"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink preserved in backup")
	}
	_ = os.RemoveAll(backup)
}

func TestIsProtected(t *testing.T) {
	cases := map[string]bool{
		"memory/foo.md":       true,
		"projects/x.jsonl":    true,
		"credentials.json":    true,
		"rules/testing.md":    false,
		"commands/sdd-new.md": false,
	}
	for path, want := range cases {
		got := IsProtected(path)
		if got != want {
			t.Errorf("%s: got %v, want %v", path, got, want)
		}
	}
}

func TestMutationProtectedPathsAreNotInstalled(t *testing.T) {
	dir := t.TempDir()
	assets := fstest.MapFS{
		"rules/ok.md":         {Data: []byte("ok\n")},
		"memory/secret.md":    {Data: []byte("SECRET\n")},
		"projects/conv.jsonl": {Data: []byte("CONV\n")},
	}
	plan, err := Preview(Request{
		ClaudePath: dir,
		Assets:     assets,
		Modules:    []string{"rules", "memory", "projects"},
		Mode:       ModeCopy,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range plan.Actions {
		if IsProtected(a.Source) {
			t.Errorf("protected path leaked into plan: %s", a.Source)
		}
	}
}
