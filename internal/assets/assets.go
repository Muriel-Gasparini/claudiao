package assets

import (
	"embed"
	"io/fs"
)

//go:embed all:files
var files embed.FS

var embeddedFS = mustSub(fs.Sub(files, "files"))

func FS() fs.FS { return embeddedFS }

func mustSub(f fs.FS, err error) fs.FS {
	if err != nil {
		panic("claudiao: assets embed broken: " + err.Error())
	}
	return f
}
