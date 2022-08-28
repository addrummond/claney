package compiler

import (
	"bytes"
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/davecgh/go-spew/spew"
)

func TestParseRoutes(t *testing.T) {
	testParseRoute(t, "/foo/bar/amp", "/ 'foo' / 'bar' / 'amp'")
	testParseRoute(t, "/foo/bar/:amp", "/ 'foo' / 'bar' / ${amp}")
	testParseRoute(t, "/foo/:bar_goo/amp", "/ 'foo' / ${bar_goo} / 'amp'")
	testParseRoute(t, "/foo/--:bar-/amp", "/ 'foo' / '--' ${bar} '-' / 'amp'")
	testParseRoute(t, "/foo/bar/amp:", "/ 'foo' / 'bar' / 'amp' ':'")
	testParseRoute(t, "/foo/bar:/amp", "/ 'foo' / 'bar' ':' / 'amp'")
	testParseRoute(t, "/foo/ba:arr/:amp", "/ 'foo' / 'ba' ${arr} / ${amp}")
	testParseRoute(t, "/foo/bar/:{amp}", "/ 'foo' / 'bar' / ${amp}")
	testParseRoute(t, "/foo/:{bar}/amp", "/ 'foo' / ${bar} / 'amp'")
	testParseRoute(t, "/foo/pre:{bar}post/amp", "/ 'foo' / 'pre' ${bar} 'post' / 'amp'")
	testParseRoute(t, "/foo/:*{bar}/amp", "/ 'foo' / ':' * '{bar}' / 'amp'")
	testParseRoute(t, "/foo/:**{bar}/amp", "/ 'foo' / $${bar} / 'amp'")
	testParseRoute(t, "/foo/:**{bar}", "/ 'foo' / $${bar}")
	testParseRoute(t, "/foo/:**bar", "/ 'foo' / $${bar}")
	testParseRoute(t, "/foo/:**bar/amp", "/ 'foo' / $${bar} / 'amp'")
	testParseRoute(t, "/foo/:#{bar}/amp", "/ 'foo' / $#{bar} / 'amp'")
	testParseRoute(t, "/foo/:#bar/amp", "/ 'foo' / $#{bar} / 'amp'")
	testParseRoute(t, "/foo/:\\#bar/amp", "/ 'foo' / ${#bar} / 'amp'")
	testParseRoute(t, "/foo/:", "/ 'foo' / ':'")
	testParseRoute(t, "/foo/:#", "/ 'foo' / ':#'")
	testParseRoute(t, "/foo/\\[", "/ 'foo' / '['")
	testParseRoute(t, "/foo\\[", "/ 'foo['")
	testParseRoute(t, "/foo/*/bar", "/ 'foo' / * / 'bar'")
	testParseRoute(t, "/foo/**/bar", "/ 'foo' / ** / 'bar'")
	testParseRoute(t, "/foo/*", "/ 'foo' / *")
	testParseRoute(t, "/foo/*", "/ 'foo' / *")
	testParseRoute(t, "/foo/*boo*", "/ 'foo' / * 'boo' *")
	testParseRoute(t, "/foo/\\*boo\\*", "/ 'foo' / '*boo*'")
	testParseRoute(t, "/foo/**", "/ 'foo' / **")
	testParseRoute(t, "/foo/\\*", "/ 'foo' / '*'")
}

func testParseRoute(t *testing.T, route, expectedOutput string) {
	output := debugPrintParsedRoute(parseRoute(route))
	if output != expectedOutput {
		t.Errorf("\nExpected:\n  %v\nGot:\n  %v\n", expectedOutput, output)
	}
}

func TestParseRouteFile(t *testing.T) {
	const routeFile = `
	users /users
	  .
	  home    /:user_id/home [] 
		profile /:user_id/profile[]
		orders  /:user_id/orders]
  	  order /:order_id

# comment on this line
	managers /managers
	  home\ page  /:manager_id/home[
		profile     [POST,GET] /:manager_id/profile [  ] # a harmless comment
		stats       \
		            /:manager_id/stats
		orders      /orders/:user_id/\
		                             :order_id
		test1       /foobar/xyz/:maguffin # [ foo, bar ] no tags here because it's a comment
		test2       /foo/bar/:#maguffin[ foo , bar ] # comment here doesn't disrupt tags; no space required before '['
		test3       [POST] /foo/bar\[foo,bar] # escape makes it part of tag
	`

	expected := []RouteFileEntry{
		{indent: 0, name: "users", pattern: []routeElement{{kind: slash, value: ""}, {kind: constant, value: "users"}}, line: 2, terminal: true, methods: map[string]struct{}{"GET": {}}, tags: make(map[string]struct{})},
		{indent: 3, name: "home", pattern: []routeElement{{kind: slash, value: ""}, {kind: parameter, value: "user_id"}, {kind: slash, value: ""}, {kind: constant, value: "home"}}, line: 4, terminal: true, methods: map[string]struct{}{"GET": {}}, tags: make(map[string]struct{})},
		{indent: 2, name: "profile", pattern: []routeElement{{kind: slash, value: ""}, {kind: parameter, value: "user_id"}, {kind: slash, value: ""}, {kind: constant, value: "profile"}}, line: 5, terminal: true, methods: map[string]struct{}{"GET": {}}, tags: make(map[string]struct{})},
		{indent: 2, name: "orders", pattern: []routeElement{{kind: slash, value: ""}, {kind: parameter, value: "user_id"}, {kind: slash, value: ""}, {kind: constant, value: "orders]"}}, line: 6, terminal: false, methods: map[string]struct{}{"GET": {}}, tags: make(map[string]struct{})},
		{indent: 5, name: "order", pattern: []routeElement{{kind: slash, value: ""}, {kind: parameter, value: "order_id"}}, line: 7, terminal: true, methods: map[string]struct{}{"GET": {}}, tags: make(map[string]struct{})},
		{indent: 0, name: "managers", pattern: []routeElement{{kind: slash, value: ""}, {kind: constant, value: "managers"}}, line: 10, terminal: false, methods: map[string]struct{}{"GET": {}}, tags: make(map[string]struct{})},
		{indent: 3, name: "home page", pattern: []routeElement{{kind: slash, value: ""}, {kind: parameter, value: "manager_id"}, {kind: slash, value: ""}, {kind: constant, value: "home["}}, line: 11, terminal: true, methods: map[string]struct{}{"GET": {}}, tags: make(map[string]struct{})},
		{indent: 2, name: "profile", pattern: []routeElement{{kind: slash, value: ""}, {kind: parameter, value: "manager_id"}, {kind: slash, value: ""}, {kind: constant, value: "profile"}}, line: 12, terminal: true, methods: map[string]struct{}{"POST": {}, "GET": {}}, tags: make(map[string]struct{})},
		{indent: 2, name: "stats", pattern: []routeElement{{kind: slash, value: ""}, {kind: parameter, value: "manager_id"}, {kind: slash, value: ""}, {kind: constant, value: "stats"}}, line: 13, terminal: true, methods: map[string]struct{}{"GET": {}}, tags: make(map[string]struct{})},
		{indent: 2, name: "orders", pattern: []routeElement{{kind: slash, value: ""}, {kind: constant, value: "orders"}, {kind: slash, value: ""}, {kind: parameter, value: "user_id"}, {kind: slash, value: ""}, {kind: parameter, value: "order_id"}}, line: 15, terminal: true, methods: map[string]struct{}{"GET": {}}, tags: make(map[string]struct{})},
		{indent: 2, name: "test1", pattern: []routeElement{{kind: slash, value: ""}, {kind: constant, value: "foobar"}, {kind: slash, value: ""}, {kind: constant, value: "xyz"}, {kind: slash, value: ""}, {kind: parameter, value: "maguffin"}}, line: 17, terminal: true, methods: map[string]struct{}{"GET": {}}, tags: make(map[string]struct{})},
		{indent: 2, name: "test2", pattern: []routeElement{{kind: slash, value: ""}, {kind: constant, value: "foo"}, {kind: slash, value: ""}, {kind: constant, value: "bar"}, {kind: slash, value: ""}, {kind: integerParameter, value: "maguffin"}}, line: 18, terminal: true, methods: map[string]struct{}{"GET": {}}, tags: map[string]struct{}{"foo": {}, "bar": {}}},
		{indent: 2, name: "test3", pattern: []routeElement{{kind: slash, value: ""}, {kind: constant, value: "foo"}, {kind: slash, value: ""}, {kind: constant, value: "bar[foo,bar]"}}, line: 19, terminal: true, methods: map[string]struct{}{"POST": {}}, tags: map[string]struct{}{}},
	}

	r, errs := ParseRouteFile(strings.NewReader(routeFile))
	if len(errs) > 0 {
		t.Errorf("%+v\n", errs)
	}

	for i := range expected {
		if !reflect.DeepEqual(expected[i], r[i]) {
			t.Errorf("At %v.\nExpected\n%v\n\nGot\n%v\n", i, spew.Sdump(expected[i]), spew.Sdump(r[i]))
		}
	}
}

func TestParseRouteFileDontAllowUnderintenting(t *testing.T) {
	const routeFile = " a /foo\nb /bar\n"

	_, errs := ParseRouteFile(strings.NewReader(routeFile))
	if len(errs) != 1 {
		t.Errorf("Expecting 1 error, got %v: %+v\n", len(errs), errs)
		return
	}

	if errs[0].Kind != IndentLessThanFirstLine {
		t.Errorf("Expected IndentLessThanFirstLine, got %+v\n", errs[0].Kind)
	}
}

func TestParseRouteFileDontAllowUnderintentingNotFooledByBlankLines(t *testing.T) {
	const routeFile = "   \n    \na /foo\nb /bar\n"

	_, errs := ParseRouteFile(strings.NewReader(routeFile))
	if len(errs) != 0 {
		t.Errorf("Expecting no errors, got %v: %+v\n", len(errs), errs)
	}
}

func TestParseRouteFileDontAllowMultipleSlashes(t *testing.T) {
	const routeFile = "a /fooo//bar"

	_, errs := ParseRouteFile(strings.NewReader(routeFile))
	if len(errs) != 1 {
		t.Errorf("Expecting 1 error, got %v: %+v\n", len(errs), errs)
		return
	}

	if errs[0].Kind != MultipleSlashesInARow {
		t.Errorf("Expected MultipleSlashesInARow, got %+v\n", errs[0].Kind)
	}
}

func TestParseRouteFileAllowWeirdRouteNames(t *testing.T) {
	const routeFile = "a:!!dsadasd}]] /fooo/bar"

	_, errs := ParseRouteFile(strings.NewReader(routeFile))
	if len(errs) != 0 {
		t.Errorf("Expecting 0 errors, got %v: %+v\n", len(errs), errs)
		return
	}
}

func TestParseRouteFileMethodParsing(t *testing.T) {
	{
		r, errs := ParseRouteFile(strings.NewReader("foo [GET,POST,PUT] /"))
		if len(errs) > 0 {
			t.Errorf("Unexpected errors [1]: %+v\n", errs)
		}
		if !reflect.DeepEqual(r, []RouteFileEntry{{indent: 0, name: "foo", pattern: []routeElement{{kind: slash, value: ""}}, line: 1, terminal: true, methods: map[string]struct{}{"GET": {}, "POST": {}, "PUT": {}}, tags: map[string]struct{}{}}}) {
			t.Errorf("Unexpected tags parse [1]: %+v\n", r)
		}
	}
	{
		r, errs := ParseRouteFile(strings.NewReader("foo [ GET , POST , PUT ] /"))
		if len(errs) > 0 {
			t.Errorf("Unexpected errors [2]: %+v\n", errs)
		}
		if !reflect.DeepEqual(r, []RouteFileEntry{{indent: 0, name: "foo", pattern: []routeElement{{kind: slash, value: ""}}, line: 1, terminal: true, methods: map[string]struct{}{"GET": {}, "POST": {}, "PUT": {}}, tags: map[string]struct{}{}}}) {
			t.Errorf("Unexpected tags parse [2]: %+v\n", r)
		}
	}
	{
		r, errs := ParseRouteFile(strings.NewReader("foo [ GET , POST , PUT, ] /"))
		if len(errs) > 0 {
			t.Errorf("Unexpected errors [3]: %+v\n", errs)
		}
		if !reflect.DeepEqual(r, []RouteFileEntry{{indent: 0, name: "foo", pattern: []routeElement{{kind: slash, value: ""}}, line: 1, terminal: true, methods: map[string]struct{}{"GET": {}, "POST": {}, "PUT": {}}, tags: map[string]struct{}{}}}) {
			t.Errorf("Unexpected tags parse [3]: %+v\n", r)
		}
	}
	{
		r, errs := ParseRouteFile(strings.NewReader("foo [] /"))
		if len(errs) > 0 {
			t.Errorf("Unexpected errors [4]: %+v\n", errs)
		}
		if !reflect.DeepEqual(r, []RouteFileEntry{{indent: 0, name: "foo", pattern: []routeElement{{kind: slash, value: ""}}, line: 1, terminal: true, methods: map[string]struct{}{"GET": {}}, tags: map[string]struct{}{}}}) {
			t.Errorf("Unexpected tags parse [4]: %+v\n", r)
		}
	}
	{
		r, errs := ParseRouteFile(strings.NewReader("foo [ ] /"))
		if len(errs) > 0 {
			t.Errorf("Unexpected errors [5]: %+v\n", errs)
		}
		if !reflect.DeepEqual(r, []RouteFileEntry{{indent: 0, name: "foo", pattern: []routeElement{{kind: slash, value: ""}}, line: 1, terminal: true, methods: map[string]struct{}{"GET": {}}, tags: map[string]struct{}{}}}) {
			t.Errorf("Unexpected tags parse [5]: %+v\n", r)
		}
	}
	{
		r, errs := ParseRouteFile(strings.NewReader("foo [  ] /"))
		if len(errs) > 0 {
			t.Errorf("Unexpected errors [6]: %+v\n", errs)
		}
		if !reflect.DeepEqual(r, []RouteFileEntry{{indent: 0, name: "foo", pattern: []routeElement{{kind: slash, value: ""}}, line: 1, terminal: true, methods: map[string]struct{}{"GET": {}}, tags: map[string]struct{}{}}}) {
			t.Errorf("Unexpected tags parse [6]: %+v\n", r)
		}
	}
	{
		_, errs := ParseRouteFile(strings.NewReader("foo [GET POST PUT] /"))
		if len(errs) != 2 {
			t.Errorf("Unexpected number of errors [7]: %+v\n", errs)
		}
		if errs[0].Kind != MissingCommaBetweenMethodNames || errs[1].Kind != MissingCommaBetweenMethodNames {
			t.Errorf("Unexpected errors [7]: %+v\n", errs)
		}
	}
	{
		_, errs := ParseRouteFile(strings.NewReader("foo [GET POST PUT /"))
		if len(errs) == 0 {
			t.Errorf("Unexpected number of errors [8]: %+v\n", errs)
		}
	}
	{
		_, errs := ParseRouteFile(strings.NewReader("foo [GET,POST,PUT /"))
		if len(errs) == 0 {
			t.Errorf("Unexpected number of errors [9]: %+v\n", errs)
		}
	}
	{
		_, errs := ParseRouteFile(strings.NewReader("foo [GET,,POST] /"))
		if len(errs) != 1 {
			t.Errorf("Unexpected number of errors [10]: %+v\n", errs)
		}
		if errs[0].Kind != TwoCommasInSequenceInMethodNames {
			t.Errorf("Unexpected errors [10]: %+v\n", errs)
		}
	}
}

// Fuzz test the parser on random binary input. This catches bugs that cause
// panics or infinite loops.
func TestParseRouteFileBinaryFuzz(t *testing.T) {
	r := rand.NewSource(1234321)
	buf := make([]byte, 1024)
	for i := 0; i < 100000; i++ {
		inp := genRandBinaryInput(r, buf)
		ParseRouteFile(bytes.NewReader(inp))
	}
}

// Fuzz test the parser on random textual input. This catches bugs that cause
// panics or infinite loops.
func TestParseRouteFileTextualFuzz(t *testing.T) {
	r := rand.NewSource(1234321)
	buf := make([]byte, 1024)
	for i := 0; i < 100000; i++ {
		inp := genRandTextualInput(r, buf)
		ParseRouteFile(bytes.NewReader(inp))
	}
}

func genRandBinaryInput(r rand.Source, output []byte) []byte {
	maxLength := len(output)
	l := int(r.Int63() % int64(maxLength))
	for i := 0; i < l; i += 4 {
		rnd := r.Int63()
		output[i] = byte(rnd & 0xFF)
		if i+1 >= len(output) {
			break
		}
		output[i+1] = byte((rnd >> 8) & 0xFF)
		if i+2 >= len(output) {
			break
		}
		output[i+2] = byte((rnd >> 16) & 0xFF)
		if i+3 >= len(output) {
			break
		}
		output[i+3] = byte((rnd >> 24) & 0xFF)
	}

	return output[:l]
}

func genRandTextualInput(r rand.Source, output []byte) []byte {
	runes := []rune{' ', ' ', ' ', ' ', '\u2009', '\n', '\n', '\n', '\n', '\r', '\t', 'a', 'd', 'e', 'k', 'm', 'n', 'z', 'A', 'B', 'D', 'X', 'Y', '1', '2', '9', '[', ']', '\\', '/', '/', ',', ':', '*', '{', '}', '日', '本'}
	maxLength := len(output) - 6 // margin for utf-8 encoding
	l := int(r.Int63() % int64(maxLength))
	for i := 0; i < l; {
		rn := runes[int(r.Int63())%len(runes)]
		w := utf8.EncodeRune(output[i:], rn)
		i += w
	}
	return output[:l]
}

func TestGetTags(t *testing.T) {
	testGetTags(t, "", []string{}, 0)
	testGetTags(t, " ", []string{}, 1)
	testGetTags(t, "/foo/bar", []string{}, 8)
	testGetTags(t, "/foo/bar []", []string{}, 8)
	testGetTags(t, "/foo/bar [    ]", []string{}, 8)
	testGetTags(t, "/foo/bar [   \\  ]", []string{" "}, 8)
	testGetTags(t, "/foo/bar [ ]", []string{}, 8)
	testGetTags(t, "/foo/bar [\\ ]", []string{" "}, 8)
	testGetTags(t, "/foo/bar [\\    ]", []string{" "}, 8)
	testGetTags(t, "/foo/bar [    \\ ]", []string{" "}, 8)
	testGetTags(t, "/foo/bar [    \\ \\   ]", []string{"  "}, 8)
	testGetTags(t, "/foo/bar [  ,  ]", []string{}, 8)
	testGetTags(t, "/foo/bar [,]", []string{}, 8)
	testGetTags(t, "/foo/bar [abcde,f]", []string{"abcde", "f"}, 8)
	testGetTags(t, "/foo/bar [\\[am\\]p]", []string{"[am]p"}, 8)
	testGetTags(t, "/foo/bar [\\]\\[]", []string{"]["}, 8)
	testGetTags(t, "/foo/bar [\\^,amp]", []string{"^", "amp"}, 8)
	testGetTags(t, "/foo/bar [\\,,\\,\\,]", []string{",", ",,"}, 8)
	testGetTags(t, "/foo/bar [^amp]", []string{"^amp"}, 8)
	testGetTags(t, "/foo/bar [\\^,^,^amp]", []string{"^", "^amp"}, 8)
	testGetTags(t, "/foo/bar [aamp]", []string{"aamp"}, 8)
	testGetTags(t, "/foo/bar [  ^,a\\ \\ amp, ^foo^, \\^,^]", []string{"^", "^foo^", "a  amp"}, 8)
	testGetTags(t, "/foo/bar [ foo , bar , amp ]", []string{"amp", "bar", "foo"}, 8)
}

func testGetTags(t *testing.T, route string, expectedTags []string, expectedOffset int) {
	tags, offset := getTags(route)
	if offset != expectedOffset {
		t.Errorf("Expected offset %v, got %v\n", expectedOffset, offset)
	}
	expectedTagsSorted := make([]string, len(expectedTags))
	copy(expectedTagsSorted, expectedTags)
	sort.Strings(expectedTagsSorted)
	tagsList := make([]string, 0)
	for t := range tags {
		tagsList = append(tagsList, t)

	}
	sort.Strings(tagsList)
	if !reflect.DeepEqual(expectedTagsSorted, tagsList) {
		t.Errorf("Expected tags %v, got %v\n", expectedTagsSorted, tagsList)
	}
}
