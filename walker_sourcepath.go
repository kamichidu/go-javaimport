package main

import (
	"os"
	"path/filepath"
)

type sourceWalker struct {
	Directory string
}

func (self *sourceWalker) Walk(c *ctx) error {
	return filepath.Walk(self.Directory, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		// TODO: waiting for implementing java parser
		return nil
	})
}

var _ walker = (*sourceWalker)(nil)
