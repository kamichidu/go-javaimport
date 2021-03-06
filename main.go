package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime/pprof"
	"sync"
	"time"

	"github.com/mattn/go-isatty"
)

var (
	appVersion = "???"
)

func walkClasspath(c *ctx, path string) error {
	errCh := make(chan error, 1)
	go func() {
		var walker walker
		switch filepath.Ext(path) {
		case ".zip", ".jar":
			walker = &jarWalker{
				Filename: path,
			}
		default:
			walker = &directoryWalker{
				Directory: path,
			}
		}
		errCh <- walker.Walk(c)
	}()
	select {
	case <-c.Done():
		return c.Err()
	case err := <-errCh:
		return err
	}
}

func walkSourcepath(c *ctx, path string) error {
	errCh := make(chan error, 1)
	go func() {
		walker := &sourceWalker{
			Directory: path,
		}
		errCh <- walker.Walk(c)
	}()
	select {
	case <-c.Done():
		return c.Err()
	case err := <-errCh:
		return err
	}
}

func run(in io.Reader, out io.Writer, errOut io.Writer, args []string) int {
	var (
		err         error
		logger      = log.New(errOut, "", 0x0)
		usePprof    bool
		verbose     bool
		sourcepath  string
		classpath   string
		showVersion bool
	)
	flag := flag.NewFlagSet(args[0], flag.ExitOnError)
	flag.StringVar(&sourcepath, "sp", "", "Source search path of directories")
	flag.StringVar(&classpath, "cp", "", "Class search path of directories and zip/jar files")
	flag.BoolVar(&usePprof, "pprof", false, "Execute with pprof")
	flag.BoolVar(&verbose, "verbose", false, "Verbose mode")
	flag.BoolVar(&showVersion, "v", false, "Show version")
	flag.Parse(args[1:])

	if showVersion {
		fmt.Fprintln(errOut, appVersion)
		return 0
	}
	if usePprof {
		var w io.Writer
		if f, err := os.Create(fmt.Sprintf("%d.prof", os.Getpid())); err == nil {
			defer f.Close()
			w = f
		} else {
			logger.Printf("Can't create file: %s", err)
			w = ioutil.Discard
		}
		if err = pprof.StartCPUProfile(w); err != nil {
			logger.Printf("Can't start profiling: %s", err)
		}
		defer pprof.StopCPUProfile()
	}

	var w io.Writer
	if f, ok := out.(*os.File); ok && isatty.IsTerminal(f.Fd()) {
		w = out
	} else {
		w = bufio.NewWriter(out)
		defer w.(*bufio.Writer).Flush()
	}

	ctx := newContext(context.Background())
	ctx.SetVerbose(verbose)
	ctx.SetEmitter(newJsonLinesEmitter(w))
	ctx.SetLogger(logger)

	startedAt := time.Now()

	wg := new(sync.WaitGroup)
	for _, path := range filepath.SplitList(sourcepath) {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			if err = walkSourcepath(ctx, path); err != nil {
				logger.Printf("Error walking with %s: %s", path, err)
			}
		}(path)
	}
	for _, path := range filepath.SplitList(classpath) {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			if err = walkClasspath(ctx, path); err != nil {
				logger.Printf("Error walking with %s: %s", path, err)
			}
		}(path)
	}
	wg.Wait()

	endedAt := time.Now()
	if verbose {
		logger.Printf("time required: %s", endedAt.Sub(startedAt))
	}

	return 0
}

func main() {
	os.Exit(run(os.Stdin, os.Stdout, os.Stderr, os.Args))
}
