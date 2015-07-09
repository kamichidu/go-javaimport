package regexp

import (
	"regexp"
	"sort"
	"strings"
)

type trie map[string]trie

func newTrie() trie {
	return make(trie)
}

type TrieOptimizer struct {
	path trie
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
		if _, exist := t[key]; !exist {
			t[key] = newTrie()
		}
		t = t[key]
	}
	t["__terminal__"] = nil
}

func (self *TrieOptimizer) Compile() (*regexp.Regexp, error) {
	return regexp.Compile(self.Re())
}

func (self *TrieOptimizer) Re() string {
	if len(self.path) == 0 {
		// this pattern is never matched
		return "^\\b\x00"
	}
	return self.re(self.path)
}

func (self *TrieOptimizer) re(path trie) string {
	keys := []string{}
	for key, _ := range path {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	if _, hasTerminalKey := path["__terminal__"]; hasTerminalKey && len(keys) == 1 {
		return ""
	}

	alt := []string{}
	cc := []string{}
	q := false

	for _, key := range keys {
		qpat := regexp.QuoteMeta(key)
		if key != "__terminal__" {
			recurse := self.re(path[key])
			if recurse != "" {
				alt = append(alt, qpat+recurse)
			} else {
				cc = append(cc, qpat)
			}
		} else {
			q = true
		}
	}

	cconly := len(alt) == 0
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
		if cconly {
			result += "?"
		} else {
			result = "(?:" + result + ")?"
		}
	}
	return result
}
