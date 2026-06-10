package installer

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInstallBinaryCopies(t *testing.T) {
	// A fake "running binary" stands in for os.Executable; we install from a
	// real file so the copy path is exercised end to end.
	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "claudiao-src")
	if err := os.WriteFile(src, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	withExecutable(t, src)

	dest := t.TempDir()
	res, err := InstallBinary(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Copied {
		t.Error("expected Copied=true on first install")
	}
	want := filepath.Join(dest, binName())
	if res.Path != want {
		t.Errorf("Path = %q, want %q", res.Path, want)
	}
	info, err := os.Stat(want)
	if err != nil {
		t.Fatalf("installed binary missing: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o755 {
		t.Errorf("installed binary mode = %v, want 0o755", info.Mode().Perm())
	}
	got, _ := os.ReadFile(want)
	if string(got) != "#!/bin/sh\necho hi\n" {
		t.Errorf("content not copied faithfully: %q", got)
	}
}

func TestInstallBinaryIdempotentWhenAlreadyInPlace(t *testing.T) {
	dest := t.TempDir()
	placed := filepath.Join(dest, binName())
	if err := os.WriteFile(placed, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	withExecutable(t, placed)

	res, err := InstallBinary(dest)
	if err != nil {
		t.Fatal(err)
	}
	if res.Copied {
		t.Error("must not copy when the running binary already lives at the destination")
	}
	if !res.AlreadyOK {
		t.Error("expected AlreadyOK=true")
	}
}

func TestInstallBinaryOverwritesStale(t *testing.T) {
	dest := t.TempDir()
	placed := filepath.Join(dest, binName())
	if err := os.WriteFile(placed, []byte("OLD VERSION"), 0o755); err != nil {
		t.Fatal(err)
	}
	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "claudiao-new")
	if err := os.WriteFile(src, []byte("NEW VERSION"), 0o755); err != nil {
		t.Fatal(err)
	}
	withExecutable(t, src)

	if _, err := InstallBinary(dest); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(placed)
	if string(got) != "NEW VERSION" {
		t.Errorf("stale binary not overwritten: %q", got)
	}
}

func TestInstallBinaryMakesPathAbsolute(t *testing.T) {
	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "claudiao-src")
	if err := os.WriteFile(src, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	withExecutable(t, src)

	// A relative destination must never produce a relative hook command.
	res, err := InstallBinary("rel-bin-dir-" + filepath.Base(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(filepath.Dir(res.Path)) })
	if !filepath.IsAbs(res.Path) {
		t.Errorf("Path must be absolute, got %q", res.Path)
	}
}

func TestInstallBinaryLeavesNoTempFile(t *testing.T) {
	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "claudiao-src")
	if err := os.WriteFile(src, []byte("payload"), 0o755); err != nil {
		t.Fatal(err)
	}
	withExecutable(t, src)
	dest := t.TempDir()
	if _, err := InstallBinary(dest); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dest)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" || (len(e.Name()) > 0 && e.Name()[0] == '.') {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

func TestCopyExecutableDoesNotFollowTmpSymlink(t *testing.T) {
	// Even if a hostile symlink with our temp prefix already exists in the
	// destination, CreateTemp's O_EXCL + random suffix must not write through
	// it onto the victim.
	dest := t.TempDir()
	victimDir := t.TempDir()
	victim := filepath.Join(victimDir, "victim")
	if err := os.WriteFile(victim, []byte("ORIGINAL"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A symlink matching the old fixed name; the new code never uses it.
	_ = os.Symlink(victim, filepath.Join(dest, ".claudiao.tmp"))

	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "src")
	if err := os.WriteFile(src, []byte("NEWBIN"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := copyExecutable(src, filepath.Join(dest, "claudiao")); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(victim)
	if string(got) != "ORIGINAL" {
		t.Errorf("victim was overwritten through a temp symlink: %q", got)
	}
}

func TestInstallBinaryRefusesWorldWritableDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("world-writable mode bits are a Unix concept")
	}
	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "src")
	if err := os.WriteFile(src, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	withExecutable(t, src)

	dest := t.TempDir()
	if err := os.Chmod(dest, 0o777); err != nil {
		t.Fatal(err)
	}
	_, err := InstallBinary(dest)
	if err == nil {
		t.Fatal("must refuse to install an executable into a world-writable dir")
	}
	if _, statErr := os.Stat(filepath.Join(dest, binName())); statErr == nil {
		t.Error("binary must not have been written to the unsafe dir")
	}
}

func TestInstallBinaryIdempotentThroughSymlinkChain(t *testing.T) {
	dest := t.TempDir()
	real := filepath.Join(dest, binName())
	if err := os.WriteFile(real, []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	// The "running binary" is a symlink to the already-installed file.
	linkDir := t.TempDir()
	link := filepath.Join(linkDir, "claudiao-link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	withExecutable(t, link)

	res, err := InstallBinary(dest)
	if err != nil {
		t.Fatal(err)
	}
	if res.Copied {
		t.Error("a symlink to the installed file is the same inode — must not re-copy")
	}
	if !res.AlreadyOK {
		t.Error("expected AlreadyOK via os.SameFile")
	}
}

func TestDefaultBinDirEnvOverride(t *testing.T) {
	t.Setenv("CLAUDIAO_BIN_DIR", "/custom/bin")
	d, err := DefaultBinDir()
	if err != nil {
		t.Fatal(err)
	}
	if d != "/custom/bin" {
		t.Errorf("env override ignored: %q", d)
	}
}

func TestDirOnPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir+string(os.PathListSeparator)+"/usr/bin")
	if !dirOnPath(dir) {
		t.Error("dir present in PATH should report OnPath=true")
	}
	t.Setenv("PATH", "/usr/bin")
	if dirOnPath(dir) {
		t.Error("dir absent from PATH should report OnPath=false")
	}
}

func TestInstallBinaryReportsOnPath(t *testing.T) {
	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "claudiao-src")
	if err := os.WriteFile(src, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	withExecutable(t, src)
	dest := t.TempDir()
	t.Setenv("PATH", dest+string(os.PathListSeparator)+"/usr/bin")

	res, err := InstallBinary(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OnPath {
		t.Error("destination is on PATH but OnPath=false")
	}
}

func TestPathHintBuildsExportSnippet(t *testing.T) {
	got := PathHint("/home/u/.local/bin")
	want := `export PATH="/home/u/.local/bin:$PATH"`
	if got != want {
		t.Errorf("PathHint = %q, want %q", got, want)
	}
}

func TestPathHintTrimsTrailingSeparator(t *testing.T) {
	// A trailing separator would produce a doubled "//" in PATH; it must be
	// stripped so the snippet is clean.
	got := PathHint("/home/u/.local/bin" + string(os.PathSeparator))
	want := `export PATH="/home/u/.local/bin:$PATH"`
	if got != want {
		t.Errorf("PathHint did not trim trailing separator: %q", got)
	}
}

func TestDefaultBinDirUsesHomeWhenEnvUnset(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CLAUDIAO_BIN_DIR", "")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	d, err := DefaultBinDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".local", "bin")
	if d != want {
		t.Errorf("DefaultBinDir = %q, want %q", d, want)
	}
}

func TestInstallBinaryRefusesWhenDestIsAFile(t *testing.T) {
	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "src")
	if err := os.WriteFile(src, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	withExecutable(t, src)

	// Point the install dir at an existing regular file: ensureSafeDir must
	// reject it because it is not a directory.
	fileDir := filepath.Join(t.TempDir(), "notadir")
	if err := os.WriteFile(fileDir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := InstallBinary(fileDir)
	if err == nil {
		t.Fatal("must refuse to install when the destination dir is a regular file")
	}
}

func TestCopyExecutableErrorsWhenSourceMissing(t *testing.T) {
	dest := t.TempDir()
	err := copyExecutable(filepath.Join(t.TempDir(), "does-not-exist"), filepath.Join(dest, "claudiao"))
	if err == nil {
		t.Error("copyExecutable must error when the source cannot be opened")
	}
	// On failure no destination file may be left behind.
	if _, statErr := os.Stat(filepath.Join(dest, "claudiao")); statErr == nil {
		t.Error("destination must not exist after a failed copy")
	}
}

func TestCopyExecutableErrorsWhenTempDirUnwritable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory write-permission bits are not enforced the same way on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("root bypasses directory permission checks")
	}
	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "src")
	if err := os.WriteFile(src, []byte("payload"), 0o755); err != nil {
		t.Fatal(err)
	}
	// The destination's parent directory is read-only, so CreateTemp cannot
	// create the temp file and copyExecutable must surface the error.
	destDir := t.TempDir()
	if err := os.Chmod(destDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(destDir, 0o755) })

	err := copyExecutable(src, filepath.Join(destDir, "claudiao"))
	if err == nil {
		t.Error("copyExecutable must error when the temp file cannot be created")
	}
}

func TestCopyExecutableSucceedsAndSetsExecutableBit(t *testing.T) {
	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "src")
	if err := os.WriteFile(src, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "claudiao")
	if err := copyExecutable(src, dest); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o755 {
		t.Errorf("copied file mode = %v, want 0o755", info.Mode().Perm())
	}
	got, _ := os.ReadFile(dest)
	if string(got) != "payload" {
		t.Errorf("content not copied: %q", got)
	}
}

func TestInstallBinaryReturnsExecPathError(t *testing.T) {
	orig := execPath
	execPath = func() (string, error) { return "", os.ErrPermission }
	t.Cleanup(func() { execPath = orig })

	if _, err := InstallBinary(t.TempDir()); err == nil {
		t.Error("InstallBinary must propagate an os.Executable error")
	}
}

func TestDirOnPathIgnoresEmptyEntries(t *testing.T) {
	dir := t.TempDir()
	// A trailing separator yields an empty PATH entry that must be skipped, not
	// matched against the resolved dir.
	t.Setenv("PATH", string(os.PathListSeparator)+"/usr/bin")
	if dirOnPath(dir) {
		t.Error("empty PATH entries must not be treated as matching the dir")
	}
}

// withExecutable points os.Executable at path for the duration of the test.
func withExecutable(t *testing.T, path string) {
	t.Helper()
	orig := execPath
	execPath = func() (string, error) { return path, nil }
	t.Cleanup(func() { execPath = orig })
}
