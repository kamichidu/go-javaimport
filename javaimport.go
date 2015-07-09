package main

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/k0kubun/pp"
	"github.com/kamichidu/go-jclass"
	"github.com/mattn/go-pubsub"
	"io"
	"log"
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

	writeQueue             = pubsub.New()
	logger     *log.Logger = nil
)

func init() {
	flag.Var(excludes, "e", "Packages will be excluded.")
	flag.Var(includes, "i", "Packages will be included.")

	pp.ColoringEnabled = false

	writeQueue.Sub(write)

	logger = log.New(os.Stderr, "", log.Lshortfile|log.LstdFlags)
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

type CacheBucket struct {
	cache [256]*CacheBucket
	value *CacheEntry
}

type Cache struct {
	buckets map[string]*CacheBucket
}

func NewCache() *Cache {
	return &Cache{
		buckets: make(map[string]*CacheBucket),
	}
}

func (self *Cache) MakeKey(source string) string {
	return url.QueryEscape(source)
}

// cacheKey : file system key
// filename : array key
func (self *Cache) GetCache(cacheKey string, fileKey string, modtime time.Time) *CacheEntry {
	bucket, exist := self.buckets[cacheKey]
	if !exist {
		return nil
	}

	bucket = self.findBucket(bucket, []rune(fileKey))
	if bucket == nil {
		return nil
	}
	panic(fmt.Errorf("%#v", bucket))
	return bucket.value
}

func (self *Cache) ReadCache(cacheKey string) {
	cacheFilename := filepath.Join(*cachedir, cacheKey)
	if _, err := os.Stat(cacheFilename); err != nil {
		return
	}

	file, err := os.Open(cacheFilename)
	if err != nil {
		return
	}
	defer file.Close()

	r := bufio.NewReader(file)
	for {
		line, err := r.ReadBytes('\n')
		if err == io.EOF {
			break
		} else if err != nil {
			logger.Println(err)
			return
		}

		entry := &CacheEntry{}
		if err := json.Unmarshal(line, entry); err == nil {
			self.buckets[cacheKey] = self.putEntry(self.buckets[cacheKey], entry)
		} else {
			logger.Println(err)
		}
	}
}

func (self *Cache) SaveCache(cacheKey string) {
	bucket, exist := self.buckets[cacheKey]
	if !exist {
		return
	}

	if stat, err := os.Stat(*cachedir); err != nil {
		os.MkdirAll(*cachedir, 0600)
	} else if !stat.IsDir() {
		logger.Printf("Expects a directory, but it's a file `%s'.", *cachedir)
		return
	}

	filename := filepath.Join(*cachedir, cacheKey)
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		logger.Println(err)
		return
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	self.writeAllBuckets(writer, bucket)
}

func (self *Cache) writeAllBuckets(w *bufio.Writer, bucket *CacheBucket) {
	if bucket.value != nil {
		bytes, err := json.Marshal(bucket.value)
		if err == nil {
			w.Write(bytes)
			w.WriteRune('\n')
		} else {
			logger.Println(err)
		}
	}
	for i := 0; i < len(bucket.cache); i++ {
		if bucket.cache[i] != nil {
			self.writeAllBuckets(w, bucket.cache[i])
		}
	}
}

func (self *Cache) PutCache(cacheKey string, entry *CacheEntry) {
	self.buckets[cacheKey] = self.putEntry(self.buckets[cacheKey], entry)
}

func (self *Cache) findBucket(bucket *CacheBucket, keys []rune) *CacheBucket {
	logger.Printf("%#v\n", bucket)
	logger.Printf("%#v\n", string(keys))
	for _, key := range keys {
		if bucket == nil {
			return nil
		}
		// assum each key's int value are between 0 to 255
		idx := int(key) % len(bucket.cache)
		bucket = bucket.cache[idx]
	}
	return bucket
}

func (self *Cache) putEntry(bucket *CacheBucket, entry *CacheEntry) *CacheBucket {
	if bucket == nil {
		bucket = &CacheBucket{}
	}
	rootBucket := bucket

	keys := []rune(entry.Filename)
	for _, key := range keys {
		// assum each key's int value are between 0 to 255
		idx := int(key) % len(bucket.cache)
		if bucket.cache[idx] == nil {
			bucket.cache[idx] = &CacheBucket{}
		}
		bucket = bucket.cache[idx]
	}
	bucket.value = entry

	return rootBucket
}

func (self *Cache) ParseWithLfs(root string, filename string) (*TypeInfo, error) {
	abspath := filepath.Join(root, filename)

	stat, err := os.Stat(abspath)
	if err != nil {
		return nil, err
	}

	cacheKey := self.MakeKey(root)
	if entry := self.GetCache(cacheKey, filename, stat.ModTime()); entry != nil {
		return entry.Value, nil
	}

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
	self.PutCache(cacheKey, entry)

	return entry.Value, nil
}

func (self *Cache) ParseWithJar(filename string, zf *zip.File) (*TypeInfo, error) {
	cacheKey := self.MakeKey(filename)
	if entry := self.GetCache(cacheKey, zf.Name, zf.ModTime()); entry != nil {
		return entry.Value, nil
	}

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
	self.PutCache(cacheKey, entry)

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
			cache.ReadCache(cache.MakeKey(path))
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
			cache.SaveCache(cache.MakeKey(path))
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
