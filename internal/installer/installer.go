package installer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Mode int

const (
	ModeCopy Mode = iota
	ModeSymlink
)

type Kind int

const (
	KindCreate Kind = iota
	KindSame
	KindDiffer
)

func (k Kind) String() string {
	switch k {
	case KindCreate:
		return "create"
	case KindSame:
		return "same"
	case KindDiffer:
		return "differ"
	}
	return "unknown"
}

type Action struct {
	Source string
	Target string
	Kind   Kind
}

type Plan struct {
	ClaudePath string
	Mode       Mode
	Actions    []Action
}

type Request struct {
	ClaudePath string
	Assets     fs.FS
	AssetsDir  string
	Modules    []string
	Mode       Mode
}

var protectedNames = map[string]bool{
	"memory":              true,
	"projects":            true,
	"credentials.json":    true,
	".credentials.json":   true,
	"settings.local.json": true,
	"todos":               true,
	"shell-snapshots":     true,
	"statsig":             true,
	"history.jsonl":       true,
}

func IsProtected(relPath string) bool {
	top := strings.SplitN(filepath.ToSlash(relPath), "/", 2)[0]
	return protectedNames[top]
}

func Preview(req Request) (*Plan, error) {
	if req.ClaudePath == "" {
		return nil, errors.New("ClaudePath is required")
	}
	if req.Assets == nil {
		return nil, errors.New("Assets fs is required")
	}
	if req.Mode == ModeSymlink && req.AssetsDir == "" {
		return nil, errors.New("AssetsDir is required for symlink mode")
	}

	modules := map[string]bool{}
	for _, m := range req.Modules {
		modules[m] = true
	}

	plan := &Plan{
		ClaudePath: req.ClaudePath,
		Mode:       req.Mode,
	}

	err := fs.WalkDir(req.Assets, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." || d.IsDir() {
			return nil
		}
		if IsProtected(path) {
			return nil
		}
		top := strings.SplitN(filepath.ToSlash(path), "/", 2)[0]
		if len(modules) > 0 && !modules[top] {
			return nil
		}
		if filepath.Base(path) == ".gitkeep" || filepath.Base(path) == "README.md" {
			return nil
		}

		target := filepath.Join(req.ClaudePath, filepath.FromSlash(path))
		kind, err := classify(req.Assets, path, target, req.Mode, req.AssetsDir)
		if err != nil {
			return err
		}

		plan.Actions = append(plan.Actions, Action{
			Source: path,
			Target: target,
			Kind:   kind,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	return plan, nil
}

func classify(assets fs.FS, source, target string, mode Mode, assetsDir string) (Kind, error) {
	info, err := os.Lstat(target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return KindCreate, nil
		}
		return 0, err
	}

	if mode == ModeSymlink {
		expected := filepath.Join(assetsDir, filepath.FromSlash(source))
		if info.Mode()&os.ModeSymlink != 0 {
			dest, err := os.Readlink(target)
			if err == nil && dest == expected {
				return KindSame, nil
			}
		}
		return KindDiffer, nil
	}

	want, err := fs.ReadFile(assets, source)
	if err != nil {
		return 0, err
	}
	got, err := os.ReadFile(target)
	if err != nil {
		return 0, err
	}
	if bytes.Equal(want, got) {
		return KindSame, nil
	}
	return KindDiffer, nil
}

func Backup(claudePath string) (string, error) {
	if _, err := os.Stat(claudePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	ts := time.Now().Format("20060102-150405")
	backup := claudePath + ".backup-" + ts
	if err := copyDir(claudePath, backup); err != nil {
		return "", fmt.Errorf("backup: %w", err)
	}
	return backup, nil
}

type ProgressFn func(done, total int, current string)

func Apply(p *Plan, req Request, progress ProgressFn) error {
	if p == nil {
		return errors.New("nil plan")
	}
	total := 0
	for _, a := range p.Actions {
		if a.Kind != KindSame {
			total++
		}
	}
	done := 0
	for _, a := range p.Actions {
		if a.Kind == KindSame {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(a.Target), 0o755); err != nil {
			return err
		}
		if err := applyOne(req, a); err != nil {
			return err
		}
		done++
		if progress != nil {
			progress(done, total, a.Target)
		}
	}
	return nil
}

func applyOne(req Request, a Action) error {
	if req.Mode == ModeSymlink {
		src := filepath.Join(req.AssetsDir, filepath.FromSlash(a.Source))
		_ = os.Remove(a.Target)
		return os.Symlink(src, a.Target)
	}
	data, err := fs.ReadFile(req.Assets, a.Source)
	if err != nil {
		return err
	}
	tmp := a.Target + ".claudiao.tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, a.Target)
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			_ = os.Remove(target)
			return os.Symlink(link, target)
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
