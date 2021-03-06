package main

import (
	"os"
	"path/filepath"

	"github.com/kamichidu/go-jclass"
)

type directoryWalker struct {
	Directory string
}

func (self *directoryWalker) Walk(c *ctx) error {
	return filepath.Walk(self.Directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			c.Logger().Printf("Skipping classpath directory %s:%s", path, err)
			return filepath.SkipDir
		} else if info.IsDir() {
			return nil
		} else if filepath.Ext(info.Name()) != ".class" {
			return nil
		}

		class, err := jclass.NewJavaClassFromFilename(path)
		if err != nil {
			c.Logger().Printf("Can't create class object: %s", err)
			return nil
		}
		c.Emitter().Emit(newTypeInfoFromJavaClass(class))
		return nil
	})
}

var _ walker = (*directoryWalker)(nil)
