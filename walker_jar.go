package main

import (
	"archive/zip"
	"path/filepath"

	"github.com/kamichidu/go-jclass"
)

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

		c.Emitter().Emit(newTypeInfoFromJavaClass(class))
	}
	return nil
}

var _ walker = (*jarWalker)(nil)
