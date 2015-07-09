package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"github.com/k0kubun/pp"
	jireg "github.com/kamichidu/go-javaimport/regexp"
	"github.com/kamichidu/go-jclass"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime/pprof"
	"strings"
	"sync"
	"time"
)

type stringSet struct {
	set []string
}

func (self *stringSet) Set(arg string) error {
	for _, pkg := range strings.Split(arg, ",") {
		self.set = append(self.set, strings.Trim(pkg, " ."))
	}
	return nil
}

func (self *stringSet) String() string {
	return "[]"
}

var _ flag.Value = (*stringSet)(nil)

var (
	debug    = flag.Bool("debug", false, "DEVELOPMENT OPTION")
	profile  = flag.Bool("profile", false, "DEVELOPMENT OPTION")
	current  = flag.String("p", "", "Current package that you're in.")
	excludes = &stringSet{}
	includes = &stringSet{}

	logger *log.Logger = nil
)

func init() {
	flag.Var(excludes, "e", "Packages will be excluded.")
	flag.Var(includes, "i", "Packages will be included.")

	pp.ColoringEnabled = false

	logger = log.New(os.Stderr, "", log.Lshortfile|log.LstdFlags)
}

type PathFilter struct {
	excludePattern *regexp.Regexp
	includePattern *regexp.Regexp
}

func newPathFilter(excludes []string, includes []string) *PathFilter {
	f := &PathFilter{}

	// ignore always
	ex := jireg.NewTrieOptimizer()
	for _, exclude := range excludes {
		prefix := strings.Replace(exclude, ".", "/", -1) + "/"
		if prefix != "/" {
			ex.Add(prefix)
		}
	}
	if *debug {
		fmt.Fprintf(os.Stderr, "exclude pattern: %s\n", ex.Re())
	}
	f.excludePattern, _ = regexp.Compile("^" + ex.Re() + "|\\$\\d+\\.class$")

	in := jireg.NewTrieOptimizer()
	for _, include := range includes {
		prefix := strings.Replace(include, ".", "/", -1) + "/"
		if prefix != "/" {
			in.Add(prefix)
		}
	}
	if *debug {
		fmt.Fprintf(os.Stderr, "include pattern: %s\n", in.Re())
	}
	f.includePattern, _ = in.Compile()
	return f
}

func (self *PathFilter) Apply(path string) bool {
	if self.excludePattern != nil && self.excludePattern.MatchString(path) {
		if self.includePattern != nil && self.includePattern.MatchString(path) {
			return true
		} else {
			return false
		}
	}
	return true
}

func ParseWithLfs(root string, filename string) (*jclass.JClass, error) {
	abspath := filepath.Join(root, filename)

	r, err := os.Open(abspath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	return jclass.NewJClass(r)
}

func ParseWithJar(filename string, zf *zip.File) (*jclass.JClass, error) {
	r, err := zf.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()

	return jclass.NewJClass(r)
}

func write(jc *jclass.JClass) {
	if *current == jc.GetPackageName() {
		return
	}
	if !jc.IsPublic() {
		return
	}
	fmt.Println(jc.GetCanonicalName())
}

func runWithLfs(path string, filter *PathFilter) {
	filepath.Walk(path, func(filename string, info os.FileInfo, err error) error {
		if filepath.Ext(filename) != ".class" {
			return nil
		}

		relpath, err := filepath.Rel(path, filename)
		if err != nil {
			panic(err)
		}
		relpath = strings.Replace(relpath, "\\", "/", -1)

		if !filter.Apply(relpath) {
			return nil
		}

		typeInfo, err := ParseWithLfs(path, relpath)
		if err != nil {
			panic(err)
		}

		write(typeInfo)

		return nil
	})
}

func runWithJar(filename string, filter *PathFilter) {
	zr, err := zip.OpenReader(filename)
	if err != nil {
		panic(err)
	}
	defer zr.Close()

	for _, zf := range zr.File {
		if filepath.Ext(zf.Name) != ".class" {
			continue
		}
		if !filter.Apply(zf.Name) {
			continue
		}

		typeInfo, err := ParseWithJar(filename, zf)
		if err != nil {
			panic(err)
		}

		write(typeInfo)
	}
}

func javaimportMain() int {
	if *profile {
		f, err := os.Create("pprof.out")
		if err != nil {
			panic(err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			panic(err)
		}
		defer pprof.StopCPUProfile()
	}

	flag.Parse()

	filter := newPathFilter(excludes.set, includes.set)
	if *debug {
		fmt.Fprintf(os.Stderr, "Exclude pattern: %s\n", filter.excludePattern.String())
		fmt.Fprintf(os.Stderr, "Include pattern: %s\n", filter.includePattern.String())
	}

	allStart := time.Now()
	var wg sync.WaitGroup
	for _, path := range flag.Args() {
		path, err := filepath.Abs(path)
		if err != nil {
			panic(err)
		}

		wg.Add(1)
		go func(path string) {
			defer wg.Done()

			if *debug {
				fmt.Fprintf(os.Stderr, "Start %s\n", path)
			}
			start := time.Now()
			switch filepath.Ext(path) {
			case ".zip":
				fallthrough
			case ".jar":
				runWithJar(path, filter)
			default:
				runWithLfs(path, filter)
			}
			duration := time.Now().Sub(start)
			if *debug {
				fmt.Fprintf(os.Stderr, "Parsing %s requires %.09f [s]\n", path, duration.Seconds())
			}
		}(path)
	}
	wg.Wait()
	allDuration := time.Now().Sub(allStart)

	if *debug {
		fmt.Fprintf(os.Stderr, "Time requires %.09f [s]\n", allDuration.Seconds())
	}

	return 0
}

func main() {
	os.Exit(javaimportMain())
}
