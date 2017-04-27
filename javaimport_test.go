package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkJar(b *testing.B) {
	if os.Getenv("JAVA_HOME") == "" {
		b.Fatal("$JAVA_HOME environment variable is not set")
	}
	for i := 0; i < b.N; i++ {
		in := new(bytes.Buffer)
		out := new(bytes.Buffer)
		errOut := new(bytes.Buffer)
		exitCode := run(in, out, errOut, []string{b.Name(), "-cp", filepath.Join(os.Getenv("JAVA_HOME"), "jre/lib/rt.jar")})
		if exitCode != 0 {
			b.Errorf("exitCode: %d", exitCode)
		}
	}
}
