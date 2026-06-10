//go:build !windows

package installer

import (
	"io/fs"
	"os"
	"syscall"
)

func ownedByCurrentUser(info fs.FileInfo) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return true // unknown platform detail: don't block on it
	}
	return int(st.Uid) == os.Getuid()
}
