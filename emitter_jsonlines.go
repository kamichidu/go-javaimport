package main

import (
	"encoding/json"
	"io"
	"sync"
)

type jsonlinesEmitter struct {
	w     *json.Encoder
	mutex *sync.Mutex
}

func (self *jsonlinesEmitter) Emit(info *typeInfo) {
	if info != nil {
		self.mutex.Lock()
		defer self.mutex.Unlock()

		self.w.Encode(info)
	}
}

var _ emitter = (*jsonlinesEmitter)(nil)

func newJsonLinesEmitter(w io.Writer) *jsonlinesEmitter {
	return &jsonlinesEmitter{
		w:     json.NewEncoder(w),
		mutex: new(sync.Mutex),
	}
}
