#!/bin/sh
# claudiao installer/updater — no Go toolchain required.
#
#   curl -fsSL https://raw.githubusercontent.com/Muriel-Gasparini/claudiao/main/install.sh | sh
#
# Env overrides:
#   CLAUDIAO_VERSION      release tag to install (default: latest)
#   CLAUDIAO_INSTALL_DIR  target directory (default: ~/.local/bin)
set -eu

REPO="Muriel-Gasparini/claudiao"

say() { printf '%s\n' "$*"; }
fail() { printf 'install.sh: %s\n' "$*" >&2; exit 1; }

if [ -z "${CLAUDIAO_INSTALL_DIR:-}" ] && [ -z "${HOME:-}" ]; then
  fail "HOME is unset — set CLAUDIAO_INSTALL_DIR to choose a target directory"
fi
INSTALL_DIR="${CLAUDIAO_INSTALL_DIR:-$HOME/.local/bin}"

command -v curl >/dev/null 2>&1 || fail "curl is required"
command -v tar >/dev/null 2>&1 || fail "tar is required"

case "$(uname -s)" in
  Linux) os="linux" ;;
  Darwin) os="macos" ;;
  *) fail "unsupported OS '$(uname -s)' — download manually from https://github.com/$REPO/releases" ;;
esac

case "$(uname -m)" in
  x86_64 | amd64) arch="x86_64" ;;
  aarch64 | arm64) arch="arm64" ;;
  *) fail "unsupported architecture '$(uname -m)' — download manually from https://github.com/$REPO/releases" ;;
esac

version="${CLAUDIAO_VERSION:-}"
if [ -z "$version" ]; then
  version=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" |
    sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)
  [ -n "$version" ] || fail "could not resolve the latest release tag from the GitHub API"
fi

# `*` in a case glob matches `/` and shell metacharacters, so the shape
# check alone would accept e.g. v9.9.9/../../attacker — which curl
# normalizes into a different repo path, defeating checksum verification.
# Restrict the character set to digits, dots, and the leading v.
case "$version" in
  *[!0-9.v]*) fail "invalid version '$version' (only digits, dots, and a leading v allowed)" ;;
  v[0-9]*.[0-9]*.[0-9]*) ;;
  *) fail "invalid version '$version' (expected e.g. v0.3.0)" ;;
esac

if command -v sha256sum >/dev/null 2>&1; then
  sha_cmd="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
  sha_cmd="shasum -a 256"
else
  fail "sha256sum or shasum is required to verify the download"
fi

archive="claudiao_${version#v}_${os}_${arch}.tar.gz"
base_url="https://github.com/$REPO/releases/download/$version"

tmp=$(mktemp -d) || fail "mktemp failed"
staged=""
trap 'rm -rf "$tmp" "${staged:-/nonexistent}"' EXIT INT TERM

say "Downloading claudiao $version ($os/$arch)..."
curl -fsSL -o "$tmp/$archive" "$base_url/$archive" ||
  fail "download failed: $base_url/$archive"
curl -fsSL -o "$tmp/checksums.txt" "$base_url/checksums.txt" ||
  fail "download failed: $base_url/checksums.txt"

expected=$(awk -v f="$archive" '$2 == f { print $1 }' "$tmp/checksums.txt")
[ -n "$expected" ] || fail "$archive not found in checksums.txt"
actual=$(cd "$tmp" && $sha_cmd "$archive" | awk '{ print $1 }')
[ "$expected" = "$actual" ] ||
  fail "checksum mismatch for $archive (expected $expected, got $actual) — aborting"

tar -xzf "$tmp/$archive" -C "$tmp" claudiao || fail "could not extract claudiao from $archive"

mkdir -p "$INSTALL_DIR" || fail "cannot create $INSTALL_DIR"
[ -w "$INSTALL_DIR" ] || fail "$INSTALL_DIR is not writable — set CLAUDIAO_INSTALL_DIR or fix permissions"

chmod 0755 "$tmp/claudiao"
# mv within the same filesystem is atomic; a hook calling the binary
# mid-update sees either the old or the new version, never a torn file.
staged="$INSTALL_DIR/.claudiao.new.$$"
cp "$tmp/claudiao" "$staged"
mv -f "$staged" "$INSTALL_DIR/claudiao"
staged=""

if [ "$os" = "macos" ] && command -v xattr >/dev/null 2>&1; then
  xattr -d com.apple.quarantine "$INSTALL_DIR/claudiao" 2>/dev/null || true
fi

say ""
say "Installed: $("$INSTALL_DIR/claudiao" version)"

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) say "Note: $INSTALL_DIR is not in your PATH." ;;
esac

say ""
say "If you use the ~/.claude modules (rules, agents, hooks), refresh them now:"
say "  $INSTALL_DIR/claudiao"
say "Re-running is safe — unchanged files are skipped, changed ones are previewed."
