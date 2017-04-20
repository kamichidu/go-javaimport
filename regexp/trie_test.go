package regexp

import (
	"testing"
)

func TestTrieOptimizerCompile(t *testing.T) {
	cases := []struct {
		Patterns []string
		Re       string
	}{
		{[]string{"hoge", "fuga", "piyo"}, "(?:fuga|hoge|piyo)"},
		{[]string{"public", "private", "pipip.$"}, "p(?:ipip\\.\\$|rivate|ublic)"},
		{[]string{"sun", "sunw", "org"}, "(?:org|sunw?)"},
	}
	for _, c := range cases {
		to := NewTrieOptimizer()
		for _, pat := range c.Patterns {
			to.Add(pat)
		}
		if to.Re() != c.Re {
			t.Errorf("Re %v, wants %v", to.Re(), c.Re)
		}
	}
}
