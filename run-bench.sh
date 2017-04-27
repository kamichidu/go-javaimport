#!/bin/sh

go test -run=NONE -bench=. $(glide novendor) | tee bench.$(date +'%s')
