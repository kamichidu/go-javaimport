package main

type walker interface {
	Walk(c *ctx) error
}
