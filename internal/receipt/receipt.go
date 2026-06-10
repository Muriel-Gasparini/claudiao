// Package receipt implements the chain of custody for adversarial reviews:
// when sdd-reviewer finishes, it records a receipt bound to the exact state
// of the working tree. At commit time the pre-bash hook recomputes the
// fingerprint — if the code changed after the review, the receipt is stale
// and the commit is blocked until a re-review.
package receipt

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Muriel-Gasparini/claudiao/internal/stats"
)

type Receipt struct {
	Repo      string    `json:"repo"`
	Hash      string    `json:"hash"`
	Reviewer  string    `json:"reviewer"`
	Summary   string    `json:"summary,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

var ErrNoReceipt = errors.New("no receipt found for this repository")
var ErrStale = errors.New("working tree changed after the review — receipt is stale")

// Fingerprint binds a receipt to the exact review target: everything that
// differs from HEAD plus the porcelain status (so new untracked files also
// invalidate it).
func Fingerprint(repoRoot string) (string, error) {
	diff, err := gitOut(repoRoot, "diff", "HEAD")
	if err != nil {
		// Repo without commits yet: fall back to status only.
		diff = ""
	}
	status, err := gitOut(repoRoot, "status", "--porcelain")
	if err != nil {
		return "", fmt.Errorf("git status: %w", err)
	}
	sum := sha256.Sum256([]byte(diff + "\x00" + status))
	return hex.EncodeToString(sum[:]), nil
}

func RepoRoot(dir string) (string, error) {
	out, err := gitOut(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func path(repoRoot string) (string, error) {
	dir, err := stats.Dir()
	if err != nil {
		return "", err
	}
	rdir := filepath.Join(dir, "receipts")
	if err := os.MkdirAll(rdir, 0o700); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(repoRoot))
	return filepath.Join(rdir, hex.EncodeToString(sum[:8])+".json"), nil
}

// Create records a receipt for the current working-tree state.
func Create(dir, reviewer, summary string) (*Receipt, error) {
	root, err := RepoRoot(dir)
	if err != nil {
		return nil, err
	}
	hash, err := Fingerprint(root)
	if err != nil {
		return nil, err
	}
	r := &Receipt{
		Repo:      root,
		Hash:      hash,
		Reviewer:  reviewer,
		Summary:   stats.Sanitize(summary, 500),
		CreatedAt: time.Now().UTC(),
	}
	p, err := path(root)
	if err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return nil, err
	}
	return r, nil
}

// Load returns the stored receipt for the repo containing dir.
func Load(dir string) (*Receipt, error) {
	root, err := RepoRoot(dir)
	if err != nil {
		return nil, err
	}
	p, err := path(root)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoReceipt
		}
		return nil, err
	}
	var r Receipt
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("corrupt receipt: %w", err)
	}
	return &r, nil
}

// Verify checks that a receipt exists and still covers the current
// working-tree state.
func Verify(dir string) (*Receipt, error) {
	r, err := Load(dir)
	if err != nil {
		return nil, err
	}
	hash, err := Fingerprint(r.Repo)
	if err != nil {
		return nil, err
	}
	if hash != r.Hash {
		return r, ErrStale
	}
	return r, nil
}

func gitOut(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
