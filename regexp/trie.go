package regexp

import (
	"regexp"
	"sort"
	"strings"
)

type trie struct {
	path map[string]*trie
}

func newTrie() *trie {
	return &trie{
		path: make(map[string]*trie),
	}
}

type TrieOptimizer struct {
	path *trie
}

func NewTrieOptimizer() *TrieOptimizer {
	return &TrieOptimizer{
		path: newTrie(),
	}
}

func (self *TrieOptimizer) Add(pat string) {
	t := self.path
	for _, ch := range pat {
		key := string(ch)
		if _, exist := t.path[key]; !exist {
			t.path[key] = newTrie()
		}
		t = t.path[key]
	}
	t.path["__terminal__"] = nil
}

func (self *TrieOptimizer) Compile() (*regexp.Regexp, error) {
	return regexp.Compile(self.Re())
}

func (self *TrieOptimizer) Re() string {
	if len(self.path.path) == 0 {
		// this pattern is never matched
		return "^\\b\x00"
	}
	return self.re(self.path.path)
}

func (self *TrieOptimizer) re(path map[string]*trie) string {
	keys := []string{}
	for key, _ := range path {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	alt := []string{}
	cc := []string{}
	q := false

	for _, key := range keys {
		qpat := regexp.QuoteMeta(key)
		if nested, exist := path[key]; exist && nested != nil {
			recurse := self.re(nested.path)
			if recurse != "" {
				alt = append(alt, qpat+recurse)
			} else {
				cc = append(cc, qpat)
			}
		} else {
			q = true
		}
	}

	if len(cc) > 0 {
		if len(cc) == 1 {
			alt = append(alt, cc[0])
		} else {
			alt = append(alt, "["+strings.Join(cc, "")+"]")
		}
	}
	result := ""
	if len(alt) == 1 {
		result = alt[0]
	} else {
		result = "(?:" + strings.Join(alt, "|") + ")"
	}
	if q {
		if len(alt) == 0 {
			result += "?"
		} else {
			result = "(?:" + result + ")?"
		}
	}
	return result
}
