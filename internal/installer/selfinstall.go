package installer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// BinResult describes what the self-install step did, so the UI can tell the
// user where the binary went and whether it is reachable on the PATH.
type BinResult struct {
	Path      string // absolute path the hooks will invoke
	Copied    bool   // false when the running binary was already at Path
	OnPath    bool   // whether Dir(Path) is in $PATH
	AlreadyOK bool   // running binary already lives at the install destination
}

// execPath is os.Executable, indirected so tests can install from a fixture.
var execPath = os.Executable

func binName() string {
	if runtime.GOOS == "windows" {
		return "claudiao.exe"
	}
	return "claudiao"
}

// DefaultBinDir is where the binary is installed so hooks have a stable path.
// ~/.local/bin is the XDG-ish convention and is on most users' PATH; override
// with CLAUDIAO_BIN_DIR.
func DefaultBinDir() (string, error) {
	if d := os.Getenv("CLAUDIAO_BIN_DIR"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "bin"), nil
}

// InstallBinary copies the currently running binary into destDir under a
// stable name and returns where it landed. It is idempotent: if the running
// binary already lives at the destination, nothing is copied. The hooks then
// reference the returned path, so they keep working regardless of where the
// installer was launched from.
func InstallBinary(destDir string) (BinResult, error) {
	src, err := execPath()
	if err != nil {
		return BinResult{}, err
	}
	src = resolve(src)

	// The destination must be absolute: a relative dir would land the binary
	// relative to the process cwd and, worse, write a relative path into the
	// hook command in settings.json that breaks once the hook runs from a
	// different cwd.
	if abs, err := filepath.Abs(destDir); err == nil {
		destDir = abs
	}
	dest := filepath.Join(destDir, binName())

	res := BinResult{Path: dest, OnPath: dirOnPath(destDir)}

	// Inode identity is exact and handles symlinks/hardlinks, unlike comparing
	// resolved path strings.
	if sameFile(src, dest) {
		res.AlreadyOK = true
		return res, nil
	}

	// This binary is invoked by every Claude Code hook. Installing it into a
	// world-writable or someone-else's directory would let a local attacker
	// replace it and gain execution in the user's session — refuse.
	if err := ensureSafeDir(destDir); err != nil {
		return BinResult{}, err
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return BinResult{}, err
	}
	if err := copyExecutable(src, dest); err != nil {
		return BinResult{}, fmt.Errorf("install binary to %s: %w", dest, err)
	}
	res.Copied = true
	res.OnPath = dirOnPath(destDir) // dir now exists; recheck
	return res, nil
}

// ensureSafeDir refuses to install into a directory another local user could
// tamper with. A non-existent dir is fine — MkdirAll creates it 0o755.
func ensureSafeDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("install dir %s is not a directory", dir)
	}
	if info.Mode().Perm()&0o002 != 0 {
		return fmt.Errorf("install dir %s is world-writable (%#o); refusing to install an executable "+
			"that hooks will run — point CLAUDIAO_BIN_DIR at a private directory", dir, info.Mode().Perm())
	}
	if !ownedByCurrentUser(info) {
		return fmt.Errorf("install dir %s is not owned by you; refusing to install an executable there", dir)
	}
	return nil
}

func sameFile(a, b string) bool {
	ai, err := os.Stat(a)
	if err != nil {
		return false
	}
	bi, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(ai, bi)
}

// copyExecutable writes src to dest atomically (tmp + rename) with 0o755.
// The temp file is created with os.CreateTemp, which opens with O_EXCL and a
// random name: a predictable, non-exclusive temp name in a shared destDir is
// a symlink-attack vector (an attacker pre-creates the temp path as a symlink
// and our O_TRUNC write lands on their target). Rename over a running binary
// is allowed on Unix — the in-use inode survives for the live process.
func copyExecutable(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.CreateTemp(filepath.Dir(dest), ".claudiao-*.tmp")
	if err != nil {
		return err
	}
	tmp := out.Name()
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Chmod(0o755); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// resolve returns the canonical path, following symlinks when possible so the
// idempotency check compares real inodes rather than symlink names.
func resolve(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

func dirOnPath(dir string) bool {
	want := resolve(dir)
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		if p == "" {
			continue
		}
		if resolve(p) == want {
			return true
		}
	}
	return false
}

// PathHint returns a shell snippet to add destDir to PATH, for the UI to show
// when the install dir is not already reachable.
func PathHint(dir string) string {
	return fmt.Sprintf(`export PATH="%s:$PATH"`, strings.TrimRight(dir, string(os.PathSeparator)))
}
