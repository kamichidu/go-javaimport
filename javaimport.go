package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"github.com/k0kubun/pp"
	jireg "github.com/kamichidu/go-javaimport/regexp"
	"github.com/kamichidu/go-jclass"
	"io"
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

type FieldInfo struct {
	Name      string      `json:"name"`
	Type      string      `json:"type"`
	Value     interface{} `json:"value"`
	Public    bool        `json:"public"`
	Protected bool        `json:"protected"`
	Private   bool        `json:"private"`
}

type MethodInfo struct {
	Name           string   `json:"name"`
	ParameterTypes []string `json:"parameter_types"`
	ReturnType     string   `json:"return_type"`
	ThrowTypes     []string `json:"throw_types"`
	Public         bool     `json:"public"`
	Protected      bool     `json:"protected"`
	Private        bool     `json:"private"`
}

type TypeInfo struct {
	Name      string        `json:"name"`
	Fields    []*FieldInfo  `json:"fields"`
	Methods   []*MethodInfo `json:"methods"`
	Public    bool          `json:"public"`
	Protected bool          `json:"protected"`
	Private   bool          `json:"private"`
}

func ParseWithLfs(root string, filename string) (*TypeInfo, error) {
	abspath := filepath.Join(root, filename)

	r, err := os.Open(abspath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	info, err := ParseFile(r)
	if err != nil {
		return nil, err
	}

	return info, nil
}

func ParseWithJar(filename string, zf *zip.File) (*TypeInfo, error) {
	r, err := zf.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()

	info, err := ParseFile(r)
	if err != nil {
		return nil, err
	}

	return info, err
}

func ParseFile(in io.Reader) (*TypeInfo, error) {
	jc, err := jclass.NewJClass(in)
	if err != nil {
		return nil, err
	}

	info := &TypeInfo{}
	info.Name = jc.GetClassName()
	// TODO
	info.Public = (jc.GetAccessFlags() | 0x0) == 0x0
	info.Protected = (jc.GetAccessFlags() | 0x0) == 0x0
	info.Private = (jc.GetAccessFlags() | 0x0) == 0x0
	for _, jf := range jc.GetFields() {
		info.Fields = append(info.Fields, &FieldInfo{
			Name:      jf.GetName(),
			Type:      jf.GetType(),
			Value:     fmt.Sprint("%v", jf.GetConstantValue()),
			Public:    (jf.GetAccessFlags() | 0x0) == 0x0,
			Protected: (jf.GetAccessFlags() | 0x0) == 0x0,
			Private:   (jf.GetAccessFlags() | 0x0) == 0x0,
		})
	}
	for _, jm := range jc.GetMethods() {
		info.Methods = append(info.Methods, &MethodInfo{
			Name:           jm.GetName(),
			ReturnType:     jm.GetReturnType(),
			ParameterTypes: jm.GetParameterTypes(),
			ThrowTypes:     make([]string, 0),
			Public:         (jm.GetAccessFlags() | 0x0) == 0x0,
			Protected:      (jm.GetAccessFlags() | 0x0) == 0x0,
			Private:        (jm.GetAccessFlags() | 0x0) == 0x0,
		})
	}
	return info, nil
}

func write(info *TypeInfo) {
	fmt.Println(info.Name)
	// bytes, err := json.Marshal(info)
	// if err != nil {
	// 	pp.Fprintln(os.Stderr, info)
	// 	panic(err)
	// }
	// fmt.Println(string(bytes))
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

		typeInfo, err := ParseWithJar(filename, zf)
		if err != nil {
			panic(err)
		}

		write(typeInfo)
	}
}

func javaimportMain() int {
	f, err := os.Create("pprof.out")
	if err != nil {
		panic(err)
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		panic(err)
	}
	defer pprof.StopCPUProfile()

	flag.Parse()

	filter := newPathFilter(excludes.set, includes.set)

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

			fmt.Fprintf(os.Stderr, "Start %s\n", path)
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
			fmt.Fprintf(os.Stderr, "Parsing %s requires %.09f [s]\n", path, duration.Seconds())
		}(path)
	}
	wg.Wait()
	allDuration := time.Now().Sub(allStart)

	fmt.Fprintf(os.Stderr, "Time requires %.09f [s]\n", allDuration.Seconds())

	return 0
}

func main() {
	os.Exit(javaimportMain())
}
