package compiler

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseJsonRoute(t *testing.T) {
	const input = `
	[
		{"name": "foo", "pattern": ["foo", "pat"]},
		[
			{"name": "foo", "pattern": ["foo", [":", "pat"]]}
		]
	]
`

	entries, errors := ParseJsonRouteFile(strings.NewReader(input), DisallowUpperCase)
	fmt.Printf("(%v) %+v\n", errors, entries)
}
