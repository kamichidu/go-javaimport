package main

type sourceWalker struct {
	Directory string
}

func (self *sourceWalker) Walk(c *ctx) error {
	return nil
}

var _ walker = (*sourceWalker)(nil)
