package compiler

import (
	"strings"
	"testing"

	"github.com/go-test/deep"
)

func TestParseJsonRoute(t *testing.T) {
	const input = `
  [
    {"name": "foo", "pattern": ["/", "foo", "/", "pat"]},
    [
      {"name": "bar", "terminal": true/*false*/, "pattern": ["foo", "/", [":", "pat"]]},
      [{
        "name": "allpatternelems",
        "terminal": true,
        "pattern": [
          "/",
          "elem1",
          [":", "param1"],
          "/",
          ["*"],
          "/",
          ["**"],
          "/",
          [":**", "param2"],
          "!/"
        ]
        //
        // Some /* comment */ lines
        //
        /* Another comment */
      }],
      [[[]]] // allowed; does nothing
    ]
  ]
`

	entries, errors := ParseJsonRouteFile(strings.NewReader(input), DisallowUpperCase)

	if len(errors) > 0 {
		t.Fatalf("Errors: %+v\n", errors)
	}

	expected := []RouteFileEntry{
		{
			indent: 0,
			name:   "foo",
			pattern: []routeElement{
				{
					kind: slash,
					line: 3,
					col:  33,
				},
				{
					kind:  constant,
					value: "foo",
					line:  3,
					col:   38,
				},
				{
					kind: slash,
					line: 3,
					col:  45,
				},
				{
					kind:  constant,
					value: "pat",
					line:  3,
					col:   50,
				},
			},
			line:     3,
			terminal: false,
			tags:     map[string]struct{}{},
			methods:  map[string]struct{}{"GET": {}},
		},
		{
			indent: 1,
			name:   "bar",
			pattern: []routeElement{
				{
					kind:  constant,
					value: "foo",
					line:  5,
					col:   62,
				},
				{
					kind: slash,
					line: 5,
					col:  69,
				},
				{
					kind:  parameter,
					value: "pat",
					line:  5,
					col:   74,
				},
			},
			line:     5,
			terminal: true,
			tags:     map[string]struct{}{},
			methods:  map[string]struct{}{"GET": {}},
		},
		{
			indent: 2,
			name:   "allpatternelems",
			pattern: []routeElement{
				{
					kind: slash,
					line: 10,
					col:  11,
				},
				{
					kind:  constant,
					value: "elem1",
					line:  11,
					col:   11,
				},
				{
					kind:  parameter,
					value: "param1",
					line:  12,
					col:   11,
				},
				{
					kind: slash,
					line: 13,
					col:  11,
				},
				{
					kind: singleGlob,
					line: 14,
					col:  11,
				},
				{
					kind: slash,
					line: 15,
					col:  11,
				},
				{
					kind: doubleGlob,
					line: 16,
					col:  11,
				},
				{
					kind: slash,
					line: 17,
					col:  11,
				},
				{
					kind:  restParameter,
					value: "param2",
					line:  18,
					col:   11,
				},
				{
					kind: noTrailingSlash,
					line: 19,
					col:  11,
				},
			},
			line:     6,
			terminal: true,
			tags:     map[string]struct{}{},
			methods:  map[string]struct{}{"GET": {}},
		},
	}

	deep.CompareUnexportedFields = true
	if diff := deep.Equal(expected, entries); diff != nil {
		t.Fatal(diff)
	}
}
