package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/k0kubun/pp"
	"github.com/kamichidu/go-jclass"
	"github.com/mattn/go-pubsub"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"sync"
	"time"
)

var _ io.Reader = nil

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
	cachedir = flag.String("c", os.ExpandEnv("$TEMP/javaimport/"), "Cache directory")
	noCache  = flag.Bool("C", false, "Disable cache feature.")
	current  = flag.String("p", "", "Current package that you're in.")
	excludes = &stringSet{}
	includes = &stringSet{}

	writeQueue = pubsub.New()
)

func init() {
	flag.Var(excludes, "e", "Packages will be excluded.")
	flag.Var(includes, "i", "Packages will be included.")

	pp.ColoringEnabled = false

	writeQueue.Sub(write)
}

type PathFilter struct {
	excludes []string
	includes []string
}

func newPathFilter(excludes []string, includes []string) *PathFilter {
	f := &PathFilter{}
	for _, exclude := range excludes {
		prefix := strings.Replace(exclude, ".", "/", -1) + "/"
		if prefix != "/" {
			f.excludes = append(f.excludes, prefix)
		}
	}
	for _, include := range includes {
		prefix := strings.Replace(include, ".", "/", -1) + "/"
		if prefix != "/" {
			f.includes = append(f.includes, prefix)
		}
	}
	return f
}

func (self *PathFilter) Apply(path string) bool {
	for _, exclude := range self.excludes {
		if strings.HasPrefix(path, exclude) {
			except := false
			for _, include := range self.includes {
				if strings.HasPrefix(path, include) {
					except = true
					break
				}
			}
			if !except {
				return false
			}
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

type CacheEntry struct {
	Filename string    `json:"filename"`
	Modtime  int64     `json:"modtime"`
	Value    *TypeInfo `json:"value"`
}

type Cache struct {
	cachedir string
	cache    map[string][]*CacheEntry
}

func (self *Cache) MakeKey(source string) string {
	return url.QueryEscape(source)
}

type CacheFileKey interface {
	Name() string
	ModTime() time.Time
}

func (self *Cache) GetCache(cacheKey string, file CacheFileKey) *CacheEntry {
	// stat, err := os.Stat(filename)
	// if err != nil {
	// 	return nil
	// }

	basename := filepath.Base(file.Name())
	pp.Errorf("basename = %s\n", basename)
	if cache, ok := self.cache[cacheKey]; ok {
		for _, entry := range cache {
			if entry.Filename == basename {
				if entry.Modtime >= file.ModTime().Unix() {
					return entry
				}
			}
		}
	}
	return nil
}

func (self *Cache) ReadCache(cacheKey string) {
	stat, err := os.Stat(filepath.Join(self.cachedir, cacheKey))
	if err != nil {
		return
	}
	b, err := ioutil.ReadFile(stat.Name())
	if err != nil {
		return
	}
	r := bufio.NewReader(bytes.NewReader(b))
	for {
		line, _, err := r.ReadLine()
		if err == nil {
			var data *CacheEntry
			if json.Unmarshal(line, data) == nil {
				self.StoreCache(cacheKey, data)
			}
		}
	}
}

func (self *Cache) StoreCache(cacheKey string, entry *CacheEntry) {
	return
	for i, old := range self.cache[cacheKey] {
		if old.Value.Name == entry.Value.Name {
			self.cache[cacheKey][i] = entry
			return
		}
	}
	self.cache[cacheKey] = append(self.cache[cacheKey], entry)
}

func (self *Cache) ParseWithLfs(root string, filename string) (*TypeInfo, error) {
	abspath := filepath.Join(root, filename)

	stat, err := os.Stat(abspath)
	if err != nil {
		return nil, err
	}

	cacheKey := self.MakeKey(root)
	// if entry := self.GetCache(cacheKey, stat); entry != nil {
	// 	return entry.Value, nil
	// } else {
	// 	self.ReadCache(cacheKey)
	// }
	// if entry := self.GetCache(cacheKey, stat); entry != nil {
	// 	return entry.Value, nil
	// }

	r, err := os.Open(abspath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	info, err := self.ParseFile(r)
	if err != nil {
		return nil, err
	}

	entry := &CacheEntry{
		Filename: filename,
		Modtime:  stat.ModTime().Unix(),
		Value:    info,
	}
	self.StoreCache(cacheKey, entry)

	return entry.Value, nil
}

func (self *Cache) ParseWithJar(filename string, zf *zip.File) (*TypeInfo, error) {
	cacheKey := self.MakeKey(filename)
	// if entry := self.GetCache(cacheKey, zf); entry != nil {
	// 	return entry.Value, nil
	// } else {
	// 	self.ReadCache(cacheKey)
	// }
	// if entry := self.GetCache(cacheKey, zf); entry != nil {
	// 	return entry.Value, nil
	// }

	r, err := zf.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()

	info, err := self.ParseFile(r)
	if err != nil {
		return nil, err
	}

	entry := &CacheEntry{
		Filename: zf.Name,
		Modtime:  zf.ModTime().Unix(),
		Value:    info,
	}
	self.StoreCache(cacheKey, entry)

	return entry.Value, err
}

func (self *Cache) ParseFile(in io.Reader) (*TypeInfo, error) {
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

func NewCache() *Cache {
	return &Cache{
		cachedir: *cachedir,
		cache:    make(map[string][]*CacheEntry),
	}
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

func runWithLfs(cache *Cache, path string, filter *PathFilter) {
	filepath.Walk(path, func(filename string, info os.FileInfo, err error) error {
		if filepath.Ext(filename) != ".class" {
			return nil
		}

		relpath, err := filepath.Rel(path, filename)
		if err != nil {
			panic(err)
		}
		relpath = strings.Replace(relpath, "\\", "/", -1)

		typeInfo, err := cache.ParseWithLfs(path, relpath)
		if err != nil {
			panic(err)
		}

		writeQueue.Pub(typeInfo)

		return nil
	})
}

func runWithJar(cache *Cache, filename string, filter *PathFilter) {
	zr, err := zip.OpenReader(filename)
	if err != nil {
		panic(err)
	}
	defer zr.Close()

	for _, zf := range zr.File {
		if filepath.Ext(zf.Name) != ".class" {
			continue
		}

		typeInfo, err := cache.ParseWithJar(filename, zf)
		if err != nil {
			panic(err)
		}

		writeQueue.Pub(typeInfo)
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
	cache := NewCache()

	var wg sync.WaitGroup
	for _, path := range flag.Args() {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()

			fmt.Fprintf(os.Stderr, "Start %s\n", path)
			start := time.Now()
			switch filepath.Ext(path) {
			case ".zip":
				fallthrough
			case ".jar":
				runWithJar(cache, path, filter)
			default:
				runWithLfs(cache, path, filter)
			}
			duration := time.Now().Sub(start)
			fmt.Fprintf(os.Stderr, "Parsing %s requires %.09f [s]\n", path, duration.Seconds())
		}(path)
	}
	wg.Wait()

	return 0
}

func main() {
	os.Exit(javaimportMain())
}
