package test

import (
	"github.com/k0kubun/pp"
	"github.com/kamichidu/go-javaimport/regexp"
	"testing"
)

func init() {
	pp.ColoringEnabled = false
}

func TestTrieOptimizer_Compile(t *testing.T) {
	to := regexp.NewTrieOptimizer()
	to.Add("hoge")
	to.Add("fuga")
	to.Add("piyo")
	if to.Re() != "(?:fuga|hoge|piyo)" {
		t.Errorf("%s vs (?:fuga|hoge|piyo)", to.Re())
	}

	to = regexp.NewTrieOptimizer()
	to.Add("public")
	to.Add("private")
	to.Add("pipip.$")
	if to.Re() != "p(?:ipip\\.\\$|rivate|ublic)" {
		t.Errorf("%s vs p(?:ipip\\.\\$|rivate|ublic)", to.Re())
	}

	to = regexp.NewTrieOptimizer()
	if to.Re() != "^\\b\x00" {
		t.Errorf("%s vs ^\\b\x00", to.Re())
	}

	to = regexp.NewTrieOptimizer()
	to.Add("sun")
	to.Add("sunw")
	to.Add("org")
	pp.Println(to)
    if to.Re() != "(?:org|sunw?)" {
        t.Errorf("%s vs (?:org|sunw?)", to.Re())
	}
}
