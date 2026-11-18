// Package fscache keeps an on-disk cache directory under a size limit.
package fscache

import (
	"os"
	"path/filepath"
	"time"
)

func EnforceLimit(dir string, limit int64) {
	if limit <= 0 {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type file struct {
		path string
		mod  time.Time
	}
	var total int64
	var files []file
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		total += info.Size()
		files = append(files, file{filepath.Join(dir, e.Name()), info.ModTime()})
	}
	if total <= limit {
		return
	}
	for i := 1; i < len(files); i++ {
		for j := i; j > 0 && files[j].mod.Before(files[j-1].mod); j-- {
			files[j], files[j-1] = files[j-1], files[j]
		}
	}
	for _, f := range files {
		if total <= limit {
			break
		}
		if info, err := os.Stat(f.path); err == nil {
			total -= info.Size()
		}
		os.Remove(f.path)
	}
}
