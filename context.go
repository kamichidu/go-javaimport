package main

import (
	"context"
	"log"
)

type ctx struct {
	context.Context
}

func newContext(parent context.Context) *ctx {
	return &ctx{parent}
}

func (self *ctx) Emitter() emitter {
	return self.Value("Emitter").(emitter)
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

func (self *ctx) SetEmitter(v emitter) {
	self.Context = context.WithValue(self.Context, "Emitter", v)
}

func (self *ctx) SetLogger(v *log.Logger) {
	self.Context = context.WithValue(self.Context, "Logger", v)
}
