package main

import (
	"context"
)

func walkSourcepath(c context.Context, path string) error {
	errCh := make(chan error, 1)

	errCh <- nil

	select {
	case <-c.Done():
		return c.Err()
	case err := <-errCh:
		return err
	}
}
