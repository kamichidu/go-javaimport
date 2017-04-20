package main

import (
	"archive/zip"
	"github.com/kamichidu/go-jclass"
	"path/filepath"
)

func walkClasspath(c *ctx, path string) error {
	errCh := make(chan error, 1)
	go func() {
		switch filepath.Ext(path) {
		case ".zip", ".jar":
			walker := &jarWalker{
				Filename: path,
			}
			errCh <- walker.Walk(c)
		default:
			walker := &directoryWalker{
				Directory: path,
			}
			errCh <- walker.Walk(c)
		}
	}()
	select {
	case <-c.Done():
		return c.Err()
	case err := <-errCh:
		return err
	}
}

type jarWalker struct {
	Filename string
}

func (self *jarWalker) Walk(c *ctx) error {
	zr, err := zip.OpenReader(self.Filename)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, zf := range zr.File {
		if filepath.Ext(zf.Name) != ".class" {
			continue
		}
		r, err := zf.Open()
		if err != nil {
			c.Logger().Printf("Can't open classfile %s:%s:%s", self.Filename, zf.Name, err)
			continue
		}
		defer r.Close()

		class, err := jclass.NewJavaClassFromReader(r)
		if err != nil {
			c.Logger().Printf("Can't parse classfile: %s:%s:%s", self.Filename, zf.Name, err)
			continue
		}

		c.Emitter().Emit(class)
	}
	return nil
}

type directoryWalker struct {
	Directory string
}

func (self *directoryWalker) Walk(c *ctx) error {
	return nil
}
