package compiler

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestNfa(t *testing.T) {
	testNfa(t, "", "", true)
	testNfa(t, "a?", "", true)
	testNfa(t, "a?", "a", true)
	testNfa(t, "a?", "aa", false)
	testNfa(t, "a+", "a", true)
	testNfa(t, "a+", "aa", true)
	testNfa(t, "a+", "", false)
	testNfa(t, "a*bc", "aabc", true)
	testNfa(t, "a*bc", "abc", true)
	testNfa(t, "a*bc", "aaabc", true)
	testNfa(t, "a*bc", "bc", true)
	testNfa(t, "ab|cd|ef", "ab", true)
	testNfa(t, "ab|cd|ef", "cd", true)
	testNfa(t, "ab|cd|ef", "ef", true)
	testNfa(t, "ab|cd|ef", "bc", false)
	testNfa(t, "(ab)|(cd)", "ab", true)
	testNfa(t, "(ab)|(cd)", "bc", false)
	testNfa(t, "(ab)|(cd)", "cd", true)
	testNfa(t, "(ab)", "ab", true)
	testNfa(t, "(ab)", "a", false)
	testNfa(t, "(ab)*", "ab", true)
	testNfa(t, "(ab)*", "abab", true)
	testNfa(t, "(ab)*", "aba", false)
	testNfa(t, "(ab)*", "ababab", true)
	testNfa(t, "(ab)*", "ababc", false)
	testNfa(t, "(ab)*c", "abc", true)
	testNfa(t, "(ab)*c", "ababc", true)
	testNfa(t, "a*", "", true)
	testNfa(t, "a*", "a", true)
	testNfa(t, "a*", "aa", true)
	testNfa(t, "a*", "aab", false)
	testNfa(t, "(a*)", "aa", true)
	testNfa(t, "(ab)*", "", true)
	testNfa(t, "(ab)*", "ab", true)
	testNfa(t, "(ab)*", "a", false)
	testNfa(t, "(ab)*", "abab", true)
	testNfa(t, "((ab))*", "abab", true)
	testNfa(t, "(a|b)*", "abab", true)
	testNfa(t, "(a|b)*", "aba", true)
	testNfa(t, "(a|b)", "a", true)
	testNfa(t, "(a|b)", "b", true)
	testNfa(t, "(a|b)*", "b", true)
	testNfa(t, "(((a|b)))*", "b", true)
	testNfa(t, "(((a|b)))*(((e|f)))", "babf", true)
	testNfa(t, "(((a|b)))*(((e|f)))", "babgf", false)
	testNfa(t, "(((a|b)))*(((e|f)))", "f", true)
	testNfa(t, "(((a|b)))*(((e|f)))", "", false)
	testNfa(t, "(((a|b)))*", "c", false)
	testNfa(t, "e|f", "", false)
	testNfa(t, "a*b", "aaab", true)
	testNfa(t, "[abcd]", "a", true)
	testNfa(t, "[abcd]", "b", true)
	testNfa(t, "[abcd]", "c", true)
	testNfa(t, "[abcd]", "d", true)
	testNfa(t, "[abcd]", "e", false)
	testNfa(t, "[^abcd]", "e", true)
	testNfa(t, "[^abcd]", "b", false)
	testNfa(t, "(a|)", "a", true)
	testNfa(t, "(a|)", "", true)
	testNfa(t, "(|a)", "a", true)
	testNfa(t, "(|a)", "", true)
	testNfa(t, "[0-9][0-9][0-9]", "234", true)
	testNfa(t, "[0-9][0-9][0-9]", "2344", false)
	testNfa(t, "[0-9][0-9][0-9]", "24", false)
	testNfa(t, "[0-9][0-9][0-9]", "aaa", false)
}

func TestNfaOverlap(t *testing.T) {
	testNfaOverlap(t, "a", "b", false)
	testNfaOverlap(t, "a", "a", true)
	testNfaOverlap(t, "a", ".", true)
	testNfaOverlap(t, "a", ".|a", true)
	testNfaOverlap(t, "(a|b)", "a", true)
	testNfaOverlap(t, "(a|b)", "b", true)
	testNfaOverlap(t, "(a|b)", "c", false)
	testNfaOverlap(t, "(a|b)*", "", true)
	testNfaOverlap(t, "(a|b)*", "a", true)
	testNfaOverlap(t, "(a|b)*", "b", true)
	testNfaOverlap(t, "(a|b)*", "c", false)
	testNfaOverlap(t, "(a|b|c|d|e)*", "abcde", true)
	testNfaOverlap(t, "(a|b|c|d|e)*", "edcba", true)
	testNfaOverlap(t, "(a|b|c|d|e)*", "abcdef", false)
	testNfaOverlap(t, "(ab|c|d|e)*", "abcde", true)
	testNfaOverlap(t, "(ab|c|d|e)*", "bacde", false)
	testNfaOverlap(t, "(ab|ccc|d|e)*", "abdeccc", true)
	testNfaOverlap(t, "(ab|ccc|d|e)*", "abdecccccc", true)
	testNfaOverlap(t, "(ab|ccc|d|e)*", "abdeccccc", false)
	testNfaOverlap(t, "(((ab|ccc|d|e)))*", "(abdecccccc)", true)
	testNfaOverlap(t, `x+a`, `x+b`, false)
	testNfaOverlap(t, `x+a`, `x+a`, true)
}

func TestFindFirstOverlap(t *testing.T) {
	testFindFirstOverlap(t, "simple", true, []string{"a", "b", "."})
	testFindFirstOverlap(t, "bigger", true, []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "."})
	testFindFirstOverlap(t, "one overlap", true, []string{"a.", "ab", "bc", "bd", "be", "bf", "bg", "bh", "bi", "bj", "bk", "bl", "bm", "xx"})
	testFindFirstOverlap(t, "many overlaps", false, []string{"xy", "ab", "bc", "bd", "be", "bf", "bg", "bh", "bi", "bj", "bk", "bl", "bm", "xx"})
}

func TestFindFirstOverlapBig(t *testing.T) {
	regexps := overlappingNeedleInHaystack(5000)
	nfa := make([]*node, len(regexps))
	for i, r := range regexps {
		c, err := regexpToNfa(r)
		if err != nil {
			t.Errorf("unexpected failure to compile %v: %v\n", r, err)
		}
		nfa[i] = c
	}
	overlaps := findOverlaps(nfa)
	if len(overlaps) != 1 {
		t.Errorf("Expected to find one overlap, got %v: %+v\n", len(overlaps), overlaps)
	}
}

func BenchFindFirstOverlapBigish(b *testing.B) {
	regexps := overlappingNeedleInHaystack(5000)
	nfa := make([]*node, len(regexps))
	for i, r := range regexps {
		c, err := regexpToNfa(r)
		if err != nil {
			b.Errorf("unexpected failure to compile %v: %v\n", r, err)
		}
		nfa[i] = c
	}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		overlaps := findOverlaps(nfa)
		if len(overlaps) != 1 {
			b.Errorf("Expected to find one overlap, got %v\n", len(overlaps))
		}
	}
}

func testNfa(t *testing.T, regexp string, input string, shouldMatch bool) {
	t.Logf("Regexp: %v\n", regexp)
	startNode, err := regexpToNfa(regexp)
	if err != nil {
		t.Errorf("Couldn't compile regexp %v\n", err)
		return
	}
	matches := run(startNode, input)
	if !matches && shouldMatch {
		t.Errorf("Expecting match – got no match: %v should match %v\n", regexp, input)
		return
	}
	if matches && !shouldMatch {
		t.Errorf("Expecting no match – got match: %v should not match %v\n", regexp, input)
		return
	}
}

func testNfaOverlap(t *testing.T, regexp1 string, regexp2 string, shouldOverlap bool) {
	startNode1, err := regexpToNfa(regexp1)
	if err != nil {
		t.Errorf("Couldn't compile regexp 1: %v\n", err)
		return
	}
	startNode2, err := regexpToNfa(regexp2)
	if err != nil {
		t.Errorf("Couldn't compile regexp 2: %v\n", err)
		return
	}
	overlaps := overlap(startNode1, startNode2)
	if !overlaps && shouldOverlap {
		t.Errorf("Expecting overlap – got no overlap: %v should overlap with %v\n", regexp1, regexp2)
		return
	}
	if overlaps && !shouldOverlap {
		t.Errorf("Expecting no overlap – got overlap: %v should not overlap with %v\n", regexp1, regexp2)
		return
	}
}

func testFindFirstOverlap(t *testing.T, desc string, overlapExists bool, regexps []string) {
	nfa := make([]*node, len(regexps))
	for i, r := range regexps {
		c, err := regexpToNfa(r)
		if err != nil {
			t.Errorf("%v: unexpected failure to compile %v: %v\n", desc, r, err)
		}
		nfa[i] = c
	}
	overlaps := findOverlaps(nfa)
	if len(overlaps) == 0 && !overlapExists {
		return
	}
	if len(overlaps) > 0 && overlapExists {
		return
	}
	t.Errorf("%v: unexpected result: overlapExists=%v overlaps=%+v\n", desc, overlapExists, overlaps)
}

func BenchmarkFindFirstOverlap10(b *testing.B) {
	benchmarkFindFirstOverlap(b, 10)
}

func BenchmarkFindFirstOverlap100(b *testing.B) {
	benchmarkFindFirstOverlap(b, 100)
}

func BenchmarkFindFirstOverlap1000(b *testing.B) {
	benchmarkFindFirstOverlap(b, 1000)
}

func BenchmarkFindFirstOverlap10000(b *testing.B) {
	benchmarkFindFirstOverlap(b, 10000)
}

func benchmarkFindFirstOverlap(b *testing.B, nRegexps int) {
	regexps := overlappingNeedleInHaystack(nRegexps)
	nfa := make([]*node, len(regexps))
	for i, r := range regexps {
		c, err := regexpToNfa(r)
		if err != nil {
			b.Errorf("unexpected failure to compile %v: %v\n", r, err)
		}
		nfa[i] = c
	}

	rand.NewSource(123)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		findOverlaps(nfa)
	}
}

// makes a list of regexps where only one pair of regexps overlaps
func overlappingNeedleInHaystack(n int) []string {
	regexps := make([]string, n)
	i := rand.Int() % len(regexps)
	var j int
	for {
		j = rand.Int() % len(regexps)
		if j != i {
			break
		}
	}

	regexps[i] = "aa"
	regexps[j] = "a*"

	for k := 0; k < n; k++ {
		if k != i && k != j {
			regexps[k] = fmt.Sprintf("%v", k)
		}
	}

	return regexps
}
