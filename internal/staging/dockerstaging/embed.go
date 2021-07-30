package dockerstaging

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

// EmbedStaging holds static files
//go:embed assets/*
var EmbedStaging embed.FS

type CallBackEmbedStaging func(fs.File, string, fs.FileMode) error

func IterateEmbedStaging(fn CallBackEmbedStaging) error {
	dir, err := EmbedStaging.ReadDir("assets")
	if err != nil {
		panic(err)
	}
	for _, f := range dir {
		entry, err := EmbedStaging.Open(filepath.Join("assets", f.Name()))
		if err != nil {
			panic(err)
		}
		fn(entry, f.Name(), os.FileMode(0755))
		entry.Close()
	}
	return nil
}
