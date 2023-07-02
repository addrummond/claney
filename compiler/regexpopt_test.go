package compiler

import (
	"strings"
	"testing"
)

func TestParseRegexp(t *testing.T) {
	type tst struct {
		regexp string
		parse  string
	}

	cases := []tst{
		{"", "''"},
		{"a", ">>\n··a"},
		{"ab", ">>\n··ab"},
		{"ab\\(", ">>\n··ab\\("},
		{"(foo|bar)|amp",
			`
|(
··>>
····(
······|(
········>>
··········foo
········>>
··········bar
······|)
····)
··>>
····amp
|)
		`},
		{"foo(|bar|)|",
			`
|(
··>>
····foo
····(
······|(
········>>
··········''
········>>
··········bar
········''
······|)
····)
··''
|)`},
		{"foo(|bar|)|||",
			`
|(
··>>
····foo
····(
······|(
········>>
··········''
········>>
··········bar
········''
······|)
····)
··>>
····''
··>>
····''
··''
|)`},
		{"(amp)foo",
			`
>>
··(
····>>
······amp
··)
··foo`},
		{"(blub)|a",
			`
|(
··>>
····(
······>>
········blub
····)
··>>
····a
|)`},
		{"(foo)|(bar)|(amp)",
			`
|(
··>>
····(
······>>
········foo
····)
··>>
····(
······>>
········bar
····)
··>>
····(
······>>
········amp
····)
|)`},
		{"((foo)|(bar))",
			`
>>
··(
····|(
······>>
········(
··········>>
············foo
········)
······>>
········(
··········>>
············bar
········)
····|)
··)
`},
		{"(?:(1)\\/*|(2)\\/*|(3)\\/*|(4)\\/*|(5))",
			`
>>
··(?:
····|(
······>>
········(
··········>>
············1
········)
········\/*
······>>
········(
··········>>
············2
········)
········\/*
······>>
········(
··········>>
············3
········)
········\/*
······>>
········(
··········>>
············4
········)
········\/*
······>>
········(
··········>>
············5
········)
····|)
··)
`},
	}

	for _, c := range cases {
		pretty := debugPrintRenode(parseRegexp(c.regexp))
		if strings.TrimSpace(pretty) != strings.TrimSpace(c.parse) {
			t.Errorf("Bad parse for %v\nExpected\n%v\n\nGot\n%v\n", c.regexp, strings.TrimSpace(c.parse), strings.TrimSpace(pretty))
		}
	}
}

func TestFindSingleGroupDisjuncts(t *testing.T) {
	type tst struct {
		input, output string
	}

	testCases := []tst{
		{"(foo)|(bar)", "(foo)|(bar)"},
		{"(foo)|(bar)|(amp)", "(foo|bar|amp)"},
		{"(foo\\/*)|(bar\\/*)|(amp\\/*)", "(foo\\/*|bar\\/*|amp\\/*)"},
		{"(((foo)|(bar)|(amp)))", "(((foo|bar|amp)))"},
		{"(foo)|(bar)|(amp)|abcd|(ef)(gh)", "(foo|bar|amp)|abcd|(ef)(gh)"},
		{"(foo)|(bar)|(amp)|abcd|(ef)(gh)|", "(foo|bar|amp)|abcd|(ef)(gh)|"},
		{"(foo)|(bar)|(amp)|abcd|(ef)(gh)||", "(foo|bar|amp)|abcd|(ef)(gh)||"},
		{"(ef)(gh)|(foo)|(bar)|(amp)|abcd", "(foo|bar|amp)|(ef)(gh)|abcd"},
		{"(ef)(gh)|(foo)|(bar)|(amp)|abcd", "(foo|bar|amp)|(ef)(gh)|abcd"},
		{"(1)a|", "(1)a|"},
		{"(?:(1)\\/*|(2)\\/*|(3)\\/*|(4)\\/*|(5))", "(?:(1|2|3|4)\\/*|(5))"},
		{"(?:(5)|(1)\\/*|(2)\\/*|(3)\\/*|(4)\\/*)", "(?:(5)|(1|2|3|4)\\/*)"},
		{"(?:(5)|(1)\\/*|(2)xx|(3)xx|(4)\\/*)", "(?:(5)|(1|4)\\/*|(2|3)xx)"},
		{"(?:(1)\\/*|(2)\\/*|(3)\\/*|(4)\\/*|(5))(?:(8)|(9)|(a)|(b)|c)", "(?:(1|2|3|4)\\/*|(5))(?:(8|9|a|b)|c)"},
	}

	for _, c := range testCases {
		n := parseRegexp(c.input)
		scratchBuffer := make([]byte, 64)
		sgds := findSingleGroupDisjuncts(n, scratchBuffer)
		refactorSingleGroupDisjuncts(sgds)
		out := renodeToString(n)
		if out != c.output {
			t.Errorf("Expected %v to go to %v, got %v\n", c.input, c.output, out)
		}
	}
}

func TestFindSingleGroupDisjunctsWorksOkIfBufferTooShort(t *testing.T) {
	type tst struct {
		input, output string
	}

	testCases := []tst{
		{"(foo)x|(bar)x|(amp)x", "(foo|bar|amp)x"},
		{"(foo)xx|(bar)xx|(amp)xx", "(foo)xx|(bar)xx|(amp)xx"},
	}

	for _, c := range testCases {
		n := parseRegexp(c.input)
		scratchBuffer := make([]byte, 1)
		sgds := findSingleGroupDisjuncts(n, scratchBuffer)
		refactorSingleGroupDisjuncts(sgds)
		out := renodeToString(n)
		if out != c.output {
			t.Errorf("Short scratch buffer: expected %v to go to %v, got %v\n", c.input, c.output, out)
		}
	}
}

func TestFindSingleGroupDisjunctsWorksOkIfBufferEmpty(t *testing.T) {
	type tst struct {
		input, output string
	}

	testCases := []tst{
		{"(foo)x|(bar)x|(amp)x", "(foo)x|(bar)x|(amp)x"},
		{"(foo)xx|(bar)xx|(amp)xx", "(foo)xx|(bar)xx|(amp)xx"},
	}

	for _, c := range testCases {
		n := parseRegexp(c.input)
		scratchBuffer := make([]byte, 0)
		sgds := findSingleGroupDisjuncts(n, scratchBuffer)
		refactorSingleGroupDisjuncts(sgds)
		out := renodeToString(n)
		if out != c.output {
			t.Errorf("Empty scratch buffer: expected %v to go to %v, got %v\n", c.input, c.output, out)
		}
	}
}
