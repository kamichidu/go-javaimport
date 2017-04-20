package main

import (
	"context"
	"log"
)

type ctx struct {
	context.Context
}

func newContext(parent context.Context, emitter *emitter, logger *log.Logger) *ctx {
	c := context.WithValue(parent, "Emitter", emitter)
	c = context.WithValue(c, "Logger", logger)
	return &ctx{c}
}

func (self *ctx) Emitter() *emitter {
	return self.Value("Emitter").(*emitter)
}

func (self *ctx) Logger() *log.Logger {
	return self.Value("Logger").(*log.Logger)
}

func (self *ctx) Verbose() bool {
	if v, ok := self.Value("Verbose").(bool); ok {
		return v
	} else {
		return false
	}
}

func (self *ctx) SetVerbose(v bool) {
	self.Context = context.WithValue(self.Context, "Verbose", v)
}
