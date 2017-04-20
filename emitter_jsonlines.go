package main

import (
	"encoding/json"
	"io"

	"github.com/kamichidu/go-jclass"
)

type jsonlinesEmitter struct {
	w *json.Encoder
}

func (self *jsonlinesEmitter) Emit(class *jclass.JavaClass) {
	// emit only importable types
	if class.IsPrivate() || class.PackageName() == "java.lang" {
		return
	}
	self.w.Encode(newTypeInfoFromJavaClass(class))
}

var _ emitter = (*jsonlinesEmitter)(nil)

func newJsonLinesEmitter(w io.Writer) *jsonlinesEmitter {
	return &jsonlinesEmitter{
		w: json.NewEncoder(w),
	}
}
