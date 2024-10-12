package compiler

import (
	"fmt"
	"testing"
)

func TestStoppingPoints(t *testing.T) {
	var root trieNode[string]
	pool := make([]trieNode[string], 0)

	addToTrie(&pool, &root, "", "empty1")
	addToTrie(&pool, &root, "foo", "foo")
	addToTrie(&pool, &root, "bar", "bar")
	addToTrie(&pool, &root, "amp", "amp")
	addToTrie(&pool, &root, "foobar", "foobar")
	addToTrie(&pool, &root, "foobarblab", "foobarblab")
	addToTrie(&pool, &root, "bbb", "bbb")
	addToTrie(&pool, &root, "", "empty2")

	stops := stoppingPoints(&root)
	t.Logf("Stopping points %+v\n", stops)

	result := fmt.Sprintf("%+v", stops)
	const expected = "[[amp empty1 empty2] [bar empty1 empty2] [bbb empty1 empty2] [foo foobar foobarblab empty1 empty2]]"
	if result != expected {
		t.Errorf("Got\n%+v\nExpected\n%+v\n", result, expected)
	}
}

func TestStoppingPointsAllEmpty(t *testing.T) {
	var root trieNode[string]
	pool := make([]trieNode[string], 0)

	addToTrie(&pool, &root, "", "empty1")
	addToTrie(&pool, &root, "", "empty2")
	addToTrie(&pool, &root, "", "empty3")

	stops := stoppingPoints(&root)
	t.Logf("Stopping points %+v\n", stops)

	result := fmt.Sprintf("%+v", stops)
	const expected = "[[empty1 empty2 empty3]]"
	if result != expected {
		t.Errorf("Got\n%+v\nExpected\n%+v\n", result, expected)
	}
}

func TestGetConstishPrefix(t *testing.T) {
	type rt struct {
		prefix   string
		allConst bool
	}

	type tst struct {
		prefixes []rt
		expected string
	}

	testCases := []tst{
		{[]rt{{"foo", true}}, "foo"},
		{[]rt{{"foo", true}, {"bar", true}}, "foo/bar"},
		{[]rt{{"foo", true}, {"amp", false}, {"bar", true}}, "foo/amp"},
	}

	for _, tc := range testCases {
		ris := make([]*CompiledRoute, 0)
		for _, p := range tc.prefixes {
			elems := []routeElement{{slash, "", 0}}
			if !p.allConst {
				elems = append(elems, routeElement{singleGlob, "", 0})
			}
			ris = append(ris, &CompiledRoute{
				Compiled: RouteRegexp{
					ConstishPrefix: p.prefix,
					Elems:          elems,
				},
			})
		}

		got := getConstishPrefix(ris[len(ris)-1], ris[0:len(ris)-1])
		if got != tc.expected {
			t.Errorf("Expected %v got %v\n", tc.expected, got)
		}
	}
}

func TestGetConstishSuffix(t *testing.T) {
	type rt struct {
		suffix   string
		allConst bool
	}

	type tst struct {
		prefixes []rt
		expected string
	}

	testCases := []tst{
		{[]rt{{"foo", true}}, "oof"},
		{[]rt{{"foo", true}, {"bar", true}}, "rab/oof"},
		{[]rt{{"foo", true}, {"amp", false}, {"bar", true}}, "rab/pma"},
	}

	for _, tc := range testCases {
		ris := make([]*CompiledRoute, 0)
		for _, p := range tc.prefixes {
			elems := []routeElement{{slash, "", 0}}
			if !p.allConst {
				elems = append(elems, routeElement{singleGlob, "", 0})
			}
			ris = append(ris, &CompiledRoute{
				Compiled: RouteRegexp{
					ConstishSuffix: p.suffix,
					Elems:          elems,
				},
			})
		}

		got := getConstishSuffix(ris[len(ris)-1], ris[0:len(ris)-1])
		if got != tc.expected {
			t.Errorf("Expected %v got %v\n", tc.expected, got)
		}
	}
}
