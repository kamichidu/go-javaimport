package main

type directoryWalker struct {
	Directory string
}

func (self *directoryWalker) Walk(c *ctx) error {
	return nil
}

var _ walker = (*directoryWalker)(nil)
