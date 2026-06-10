//go:build windows

package installer

import "io/fs"

// Windows ownership/ACL semantics differ from Unix mode bits; we do not block
// on ownership there.
func ownedByCurrentUser(info fs.FileInfo) bool { return true }
