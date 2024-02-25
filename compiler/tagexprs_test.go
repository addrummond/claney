package compiler

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseTagExpr(t *testing.T) {
	type testcase struct {
		input, status, output string
	}

	cases := []testcase{
		{"foo", "ok", "foo"},
		{"!foo", "ok", "!foo"},
		{"!!foo", "ok", "foo"},
		{"!!!foo", "ok", "!foo"},
		{"\\ffoo", "ok", "ffoo"},
		{"\\*foo", "ok", "\\*foo"}, // \* is special cased for easier escaping of globs
		{"(foo-\\*-bar)", "ok", "foo-\\*-bar"},
		{"foo & bar", "ok", "(foo&bar)"},
		{"(foo & bar)", "ok", "(foo&bar)"},
		{"(foo)", "ok", "foo"},
		{"(foo-*)", "ok", "foo-*"},
		{"(foo-\\*)", "ok", "foo-\\*"},
		{"((foo) ) ", "ok", "foo"},
		{"( ((foo) ) )  ", "ok", "foo"},
		{"( ((foo) )   ", "1:13: No closing ')' found", ""},
		{"  ((foo) ) ) ", "1:13: Trailing input: ') '", ""},
		{"foo & bar | amp & baz & fug", "ok", "((((foo&bar)|amp)&baz)&fug)"},
		{"[foo]", "ok", "[FOO]"},
		{"[foo] & [bar  ]& amp&[   goo]", "ok", "((([FOO]&[BAR])&amp)&[GOO])"},
		{"[foo] & bar", "ok", "([FOO]&bar)"},
		{"[b ar]", "ok", "[B AR]"},
		{"[\\ \\[b\\] ar]", "ok", "[[B] AR]"},
		{"foo & [ ba r ] | amp & [baz] & fug", "ok", "((((foo&[BA R])|amp)&[BAZ])&fug)"},
		{"foo\\ \\&\\ \\[\\ ba\\ r\\ \\]\\ \\|\\ amp\\ \\&\\ \\[baz\\]\\ \\&\\ fug", "ok", "foo & [ ba r ] | amp & [baz] & fug"},
	}

	for _, tc := range cases {
		expr, err := ParseTagExpr(tc.input)
		if err == nil && tc.status != "ok" {
			t.Errorf("Expected error when parsing %v, got %v\n", tc.input, debugPrintExpr(expr))
		} else if err != nil && tc.status == "ok" {
			t.Errorf("Expected %v when parsing %v, got %v\n", tc.output, tc.input, err)
		} else if err != nil && tc.status != "ok" && err.Error() != tc.status {
			t.Errorf("Expected error %v when parsing %v, got %v\n", tc.status, tc.input, err.Error())
		} else if err == nil && tc.status == "ok" && tc.output != debugPrintExpr(expr) {
			t.Errorf("Expected %v when parsing %v, got %v\n", tc.output, tc.input, debugPrintExpr(expr))
		}
	}
}

func TestEvalTagExpr(t *testing.T) {
	type testcase struct {
		matches bool
		input   string
		tags    string
		methods string
	}

	cases := []testcase{
		{true, "api", "api", ""},
		{true, "api-*", "api-a", ""},
		{true, "api-*", "api-", ""},
		{false, "api", "xpi", ""},
		{false, "api-*", "xpi-a", ""},
		{false, "api-*", "xpi-", ""},
		{true, "api-\\*", "api-*", ""},
		{true, "[GET]|[P*]", "", "PUT,POST"},
		{true, "[GET]|[P*]", "", "GET"},
		{true, "[GET]|[P\\*]", "", "P*"},
		{false, "P*", "", "GET,DELETE"},
		{true, "[post]", "", "POST"},
	}

	for _, tc := range cases {
		tags := make(map[string]struct{})
		methods := make(map[string]struct{})
		for _, t := range strings.Split(tc.tags, ",") {
			tags[t] = struct{}{}
		}
		for _, t := range strings.Split(tc.methods, ",") {
			methods[t] = struct{}{}
		}
		expr, err := ParseTagExpr(tc.input)
		fmt.Printf("DE: %v\n", debugPrintExpr(expr))
		if err != nil {
			t.Fatal(err)
		}
		result := EvalTagExpr(expr, tags, methods)
		if result != tc.matches {
			t.Errorf("Expecting %v for %v, got %v", match(tc.matches), tc.input, match(result))
		}
	}
}

func match(val bool) string {
	if val {
		return "match"
	}
	return "no match"
}

func debugPrintExpr(expr *TagExpr) string {
	if expr == nil {
		return ""
	}
	switch expr.kind {
	case tagExprLiteralTag, tagExprGlobTag:
		return expr.val
	case tagExprLiteralMethod, tagExprGlobMethod:
		return "[" + expr.val + "]"
	case tagExprAnd:
		return "(" + debugPrintExpr(expr.children[0]) + "&" + debugPrintExpr(expr.children[1]) + ")"
	case tagExprOr:
		return "(" + debugPrintExpr(expr.children[0]) + "|" + debugPrintExpr(expr.children[1]) + ")"
	case tagExprNot:
		return "!" + debugPrintExpr(expr.children[0])
	}
	panic("Bad expr")
}
