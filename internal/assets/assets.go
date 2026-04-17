package assets

import (
	"embed"
	"io/fs"
)

//go:embed all:files
var embedded embed.FS

func FS() fs.FS {
	sub, err := fs.Sub(embedded, "files")
	if err != nil {
		panic(err)
	}
	return sub
}
