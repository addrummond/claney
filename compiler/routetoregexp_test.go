package compiler

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestRouteToRegexp(t *testing.T) {
	ri := routeToRegexps(parseRoute("/foo/:bar/amp"))
	if !(reflect.DeepEqual(ri.elems, []routeElement{{constant, "foo"}, {slash, ""}, {parameter, "bar"}, {slash, ""}, {constant, "amp"}}) &&
		ri.constantPortion == "foo//amp" && ri.nGroups == 1 &&
		reflect.DeepEqual(ri.paramGroupNumbers, map[string]int{"bar": 1})) {
		t.Errorf("Unexpected return value of routeToRegexps: %+v\n", ri)
	}
}

func TestProcessRouteFileNoDuplicateRouteNamesOkCase(t *testing.T) {
	const routeFile = "a /foo\na /foo/:param\n\n\na /foo/bar/amp"
	r, errs := ParseRouteFile(strings.NewReader(routeFile))
	if len(errs) > 0 {
		t.Errorf("%+v\n", errs)
	}
	_, errs = ProcessRouteFile([][]RouteFileEntry{r}, []string{""}, "/", func([]RouteWithParents) {})
	if len(errs) != 0 {
		t.Errorf("Expected no errors, got %+v\n", errs)
	}
}

func TestProcessRouteFileNoDuplicateRouteNamesBadDuplicate(t *testing.T) {
	const routeFile = "a /foo\na /foo/:param\notra /zzz/foo\n\na /foo/bar/amp"
	r, errs := ParseRouteFile(strings.NewReader(routeFile))
	if len(errs) > 0 {
		t.Errorf("%+v\n", errs)
	}
	_, errs = ProcessRouteFile([][]RouteFileEntry{r}, []string{""}, "/", func([]RouteWithParents) {})
	if len(errs) != 1 {
		t.Errorf("Expected one error, got %+v\n", errs)
		return
	}

	if errs[0].Kind != DuplicateRouteName {
		t.Errorf("Expected IndentLessThanFirstLine, got %+v\n", errs[0].Kind)
	}
	if fmt.Sprintf("%v", errs[0]) != "(line 2; line 5): two non-adjacently-nestled routes have the same name ('a'); move them next to each other in the same file" {
		t.Errorf("Error message not as expected. Got %v\n", errs[0])
	}
}

func TestRegexGeneration(t *testing.T) {
	const routeFile = `
		foo /foo
		  .
		  opt1 /opt1
			opt2 /opt2
		`

	r, errs := ParseRouteFile(strings.NewReader(routeFile))
	if len(errs) > 0 {
		t.Errorf("%+v\n", errs)
	}
	routes, errs := ProcessRouteFile([][]RouteFileEntry{r}, []string{""}, "/", func([]RouteWithParents) {})
	if len(errs) != 0 {
		t.Errorf("%+v\n", errs)
	}
	rrs := GetRouteRegexps(routes)
	fmt.Printf("%v\n", rrs.constantPortionRegexp)
}

func TestProcessRouteFile(t *testing.T) {
	const routeFile = `
	users /users
	  .
	  home    :user_id/home # initial / optional for nested route
		profile /:user_id/profile
		orders  :user_id/orders # initial / optional for nested route
  	  order /:order_id

	managers /managers
	  .
	  home    /:manager_id/home
		profile /:manager_id/profile
		stats   /:manager_id/stats
		orders  /orders/:user_id/:order_id
		test1   /foobar/xyz/:maguffin
		test2   /foo/bar/:maguffin

	dupl /
	  .
	  a /foo/bar/:param
		b /foobar/:param
		c /f/oo/bar/:param
		d /fooba/r/:param
		e /f/oobar/:param
	`

	r, errs := ParseRouteFile(strings.NewReader(routeFile))
	if len(errs) > 0 {
		t.Errorf("%+v\n", errs)
	}
	routes, errs := ProcessRouteFile([][]RouteFileEntry{r}, []string{""}, "/", func([]RouteWithParents) {})
	if len(errs) != 0 {
		t.Errorf("%+v\n", errs)
	}
	rrs := GetRouteRegexps(routes)

	const expected = `
constantPortionRegexp="^(?:\\/+(?:(?:[\\/]*|(?:(f)(\\/)[\\/]*(oo)(\\/)[\\/]*(bar)(\\/)[\\/]*[^\\/?#]+[\\/]*|(f)(\\/)[\\/]*(oobar)(\\/)[\\/]*[^\\/?#]+[\\/]*|(foo)(\\/)[\\/]*(bar)(\\/)[\\/]*[^\\/?#]+[\\/]*|(fooba)(\\/)[\\/]*(r)(\\/)[\\/]*[^\\/?#]+[\\/]*|(foobar)(\\/)[\\/]*[^\\/?#]+[\\/]*))|(managers)(?:[\\/]*|(\\/)[\\/]*(?:[^\\/?#]+(\\/)[\\/]*(home)[\\/]*|[^\\/?#]+(\\/)[\\/]*(profile)[\\/]*|[^\\/?#]+(\\/)[\\/]*(stats)[\\/]*|(?:(f)(?:(oo)(\\/)[\\/]*(bar)(\\/)[\\/]*[^\\/?#]+[\\/]*|(oobar)(\\/)[\\/]*(xyz)(\\/)[\\/]*[^\\/?#]+[\\/]*))||(orders)(\\/)[\\/]*[^\\/?#]+(\\/)[\\/]*[^\\/?#]+[\\/]*))|(users)(?:[\\/]*|(\\/)[\\/]*(?:[^\\/?#]+(\\/)[\\/]*(home)[\\/]*|[^\\/?#]+(\\/)[\\/]*(profile)[\\/]*|[^\\/?#]+(\\/)[\\/]*(orders)(?:(\\/)[\\/]*(?:[^\\/?#]+[\\/]*))))))(?:\\?[^#]*)?(?:#.*)?$"
constantPortionNGroups=49
families=
<families>
constantPortion=""
members=
<route-group-members>name="dupl"
route=
<route-with-parents>
name="dupl"
elems=
matchRegexp=""
constantPortionRegexp=""
constantPortion=""
constishPrefix=""
nGroups=0
paramGroupNumbers=
tags=[]
methods=[GET]
depth=0
terminal=true
line=18
</route-with-parents>
paramGroupNumbers=
</route-group-members>
nonparamGroupNumbers[1]
nLevels1
matchRegexp="^(?:(\\/+[\\/]*))(\\?[^#]*)?(#.*)?$"
constantPortion="f/oo/bar/"
members=
<route-group-members>name="dupl/c"
route=
<route-with-parents>
name="dupl/c"
elems='f' / 'oo' / 'bar' / ${param}
matchRegexp="f\\/+oo\\/+bar\\/+([^\\/?#]+)"
constantPortionRegexp="(f)(\\/)[\\/]*(oo)(\\/)[\\/]*(bar)(\\/)[\\/]*[^\\/?#]+"
constantPortion="f/oo/bar/"
constishPrefix="f/oo/bar/"
nGroups=1
paramGroupNumbers=param: 1
tags=[]
methods=[GET]
depth=1
terminal=true
line=22
</route-with-parents>
paramGroupNumbers=param: 2
</route-group-members>
nonparamGroupNumbers[1]
nLevels1
matchRegexp="^(?:(\\/+f\\/+oo\\/+bar\\/+([^\\/?#]+)[\\/]*))(\\?[^#]*)?(#.*)?$"
constantPortion="f/oobar/"
members=
<route-group-members>name="dupl/e"
route=
<route-with-parents>
name="dupl/e"
elems='f' / 'oobar' / ${param}
matchRegexp="f\\/+oobar\\/+([^\\/?#]+)"
constantPortionRegexp="(f)(\\/)[\\/]*(oobar)(\\/)[\\/]*[^\\/?#]+"
constantPortion="f/oobar/"
constishPrefix="f/oobar/"
nGroups=1
paramGroupNumbers=param: 1
tags=[]
methods=[GET]
depth=1
terminal=true
line=24
</route-with-parents>
paramGroupNumbers=param: 2
</route-group-members>
nonparamGroupNumbers[1]
nLevels1
matchRegexp="^(?:(\\/+f\\/+oobar\\/+([^\\/?#]+)[\\/]*))(\\?[^#]*)?(#.*)?$"
constantPortion="foo/bar/"
members=
<route-group-members>name="dupl/a"
route=
<route-with-parents>
name="dupl/a"
elems='foo' / 'bar' / ${param}
matchRegexp="foo\\/+bar\\/+([^\\/?#]+)"
constantPortionRegexp="(foo)(\\/)[\\/]*(bar)(\\/)[\\/]*[^\\/?#]+"
constantPortion="foo/bar/"
constishPrefix="foo/bar/"
nGroups=1
paramGroupNumbers=param: 1
tags=[]
methods=[GET]
depth=1
terminal=true
line=20
</route-with-parents>
paramGroupNumbers=param: 2
</route-group-members>
nonparamGroupNumbers[1]
nLevels1
matchRegexp="^(?:(\\/+foo\\/+bar\\/+([^\\/?#]+)[\\/]*))(\\?[^#]*)?(#.*)?$"
constantPortion="fooba/r/"
members=
<route-group-members>name="dupl/d"
route=
<route-with-parents>
name="dupl/d"
elems='fooba' / 'r' / ${param}
matchRegexp="fooba\\/+r\\/+([^\\/?#]+)"
constantPortionRegexp="(fooba)(\\/)[\\/]*(r)(\\/)[\\/]*[^\\/?#]+"
constantPortion="fooba/r/"
constishPrefix="fooba/r/"
nGroups=1
paramGroupNumbers=param: 1
tags=[]
methods=[GET]
depth=1
terminal=true
line=23
</route-with-parents>
paramGroupNumbers=param: 2
</route-group-members>
nonparamGroupNumbers[1]
nLevels1
matchRegexp="^(?:(\\/+fooba\\/+r\\/+([^\\/?#]+)[\\/]*))(\\?[^#]*)?(#.*)?$"
constantPortion="foobar/"
members=
<route-group-members>name="dupl/b"
route=
<route-with-parents>
name="dupl/b"
elems='foobar' / ${param}
matchRegexp="foobar\\/+([^\\/?#]+)"
constantPortionRegexp="(foobar)(\\/)[\\/]*[^\\/?#]+"
constantPortion="foobar/"
constishPrefix="foobar/"
nGroups=1
paramGroupNumbers=param: 1
tags=[]
methods=[GET]
depth=1
terminal=true
line=21
</route-with-parents>
paramGroupNumbers=param: 2
</route-group-members>
nonparamGroupNumbers[1]
nLevels1
matchRegexp="^(?:(\\/+foobar\\/+([^\\/?#]+)[\\/]*))(\\?[^#]*)?(#.*)?$"
constantPortion="managers"
members=
<route-group-members>name="managers"
route=
<route-with-parents>
name="managers"
elems='managers'
matchRegexp="managers"
constantPortionRegexp="(managers)"
constantPortion="managers"
constishPrefix="managers"
nGroups=0
paramGroupNumbers=
tags=[]
methods=[GET]
depth=0
terminal=true
line=9
</route-with-parents>
paramGroupNumbers=
</route-group-members>
nonparamGroupNumbers[1]
nLevels1
matchRegexp="^(?:(\\/+managers[\\/]*))(\\?[^#]*)?(#.*)?$"
constantPortion="managers//home"
members=
<route-group-members>name="managers/home"
route=
<route-with-parents>
name="managers/home"
elems=${manager_id} / 'home'
matchRegexp="([^\\/?#]+)\\/+home"
constantPortionRegexp="[^\\/?#]+(\\/)[\\/]*(home)"
constantPortion="/home"
constishPrefix=""
nGroups=1
paramGroupNumbers=manager_id: 1
tags=[]
methods=[GET]
depth=1
terminal=true
line=11
</route-with-parents>
paramGroupNumbers=manager_id: 2
</route-group-members>
nonparamGroupNumbers[1]
nLevels1
matchRegexp="^(?:(\\/+managers\\/+([^\\/?#]+)\\/+home[\\/]*))(\\?[^#]*)?(#.*)?$"
constantPortion="managers//profile"
members=
<route-group-members>name="managers/profile"
route=
<route-with-parents>
name="managers/profile"
elems=${manager_id} / 'profile'
matchRegexp="([^\\/?#]+)\\/+profile"
constantPortionRegexp="[^\\/?#]+(\\/)[\\/]*(profile)"
constantPortion="/profile"
constishPrefix=""
nGroups=1
paramGroupNumbers=manager_id: 1
tags=[]
methods=[GET]
depth=1
terminal=true
line=12
</route-with-parents>
paramGroupNumbers=manager_id: 2
</route-group-members>
nonparamGroupNumbers[1]
nLevels1
matchRegexp="^(?:(\\/+managers\\/+([^\\/?#]+)\\/+profile[\\/]*))(\\?[^#]*)?(#.*)?$"
constantPortion="managers//stats"
members=
<route-group-members>name="managers/stats"
route=
<route-with-parents>
name="managers/stats"
elems=${manager_id} / 'stats'
matchRegexp="([^\\/?#]+)\\/+stats"
constantPortionRegexp="[^\\/?#]+(\\/)[\\/]*(stats)"
constantPortion="/stats"
constishPrefix=""
nGroups=1
paramGroupNumbers=manager_id: 1
tags=[]
methods=[GET]
depth=1
terminal=true
line=13
</route-with-parents>
paramGroupNumbers=manager_id: 2
</route-group-members>
nonparamGroupNumbers[1]
nLevels1
matchRegexp="^(?:(\\/+managers\\/+([^\\/?#]+)\\/+stats[\\/]*))(\\?[^#]*)?(#.*)?$"
constantPortion="managers/foo/bar/"
members=
<route-group-members>name="managers/test2"
route=
<route-with-parents>
name="managers/test2"
elems='foo' / 'bar' / ${maguffin}
matchRegexp="foo\\/+bar\\/+([^\\/?#]+)"
constantPortionRegexp="(foo)(\\/)[\\/]*(bar)(\\/)[\\/]*[^\\/?#]+"
constantPortion="foo/bar/"
constishPrefix="foo/bar/"
nGroups=1
paramGroupNumbers=maguffin: 1
tags=[]
methods=[GET]
depth=1
terminal=true
line=16
</route-with-parents>
paramGroupNumbers=maguffin: 2
</route-group-members>
nonparamGroupNumbers[1]
nLevels1
matchRegexp="^(?:(\\/+managers\\/+foo\\/+bar\\/+([^\\/?#]+)[\\/]*))(\\?[^#]*)?(#.*)?$"
constantPortion="managers/foobar/xyz/"
members=
<route-group-members>name="managers/test1"
route=
<route-with-parents>
name="managers/test1"
elems='foobar' / 'xyz' / ${maguffin}
matchRegexp="foobar\\/+xyz\\/+([^\\/?#]+)"
constantPortionRegexp="(foobar)(\\/)[\\/]*(xyz)(\\/)[\\/]*[^\\/?#]+"
constantPortion="foobar/xyz/"
constishPrefix="foobar/xyz/"
nGroups=1
paramGroupNumbers=maguffin: 1
tags=[]
methods=[GET]
depth=1
terminal=true
line=15
</route-with-parents>
paramGroupNumbers=maguffin: 2
</route-group-members>
nonparamGroupNumbers[1]
nLevels1
matchRegexp="^(?:(\\/+managers\\/+foobar\\/+xyz\\/+([^\\/?#]+)[\\/]*))(\\?[^#]*)?(#.*)?$"
constantPortion="managers/orders//"
members=
<route-group-members>name="managers/orders"
route=
<route-with-parents>
name="managers/orders"
elems='orders' / ${user_id} / ${order_id}
matchRegexp="orders\\/+([^\\/?#]+)\\/+([^\\/?#]+)"
constantPortionRegexp="(orders)(\\/)[\\/]*[^\\/?#]+(\\/)[\\/]*[^\\/?#]+"
constantPortion="orders//"
constishPrefix="orders/"
nGroups=2
paramGroupNumbers=order_id: 2, user_id: 1
tags=[]
methods=[GET]
depth=1
terminal=true
line=14
</route-with-parents>
paramGroupNumbers=order_id: 3, user_id: 2
</route-group-members>
nonparamGroupNumbers[1]
nLevels1
matchRegexp="^(?:(\\/+managers\\/+orders\\/+([^\\/?#]+)\\/+([^\\/?#]+)[\\/]*))(\\?[^#]*)?(#.*)?$"
constantPortion="users"
members=
<route-group-members>name="users"
route=
<route-with-parents>
name="users"
elems='users'
matchRegexp="users"
constantPortionRegexp="(users)"
constantPortion="users"
constishPrefix="users"
nGroups=0
paramGroupNumbers=
tags=[]
methods=[GET]
depth=0
terminal=true
line=2
</route-with-parents>
paramGroupNumbers=
</route-group-members>
nonparamGroupNumbers[1]
nLevels1
matchRegexp="^(?:(\\/+users[\\/]*))(\\?[^#]*)?(#.*)?$"
constantPortion="users//home"
members=
<route-group-members>name="users/home"
route=
<route-with-parents>
name="users/home"
elems=${user_id} / 'home'
matchRegexp="([^\\/?#]+)\\/+home"
constantPortionRegexp="[^\\/?#]+(\\/)[\\/]*(home)"
constantPortion="/home"
constishPrefix=""
nGroups=1
paramGroupNumbers=user_id: 1
tags=[]
methods=[GET]
depth=1
terminal=true
line=4
</route-with-parents>
paramGroupNumbers=user_id: 2
</route-group-members>
nonparamGroupNumbers[1]
nLevels1
matchRegexp="^(?:(\\/+users\\/+([^\\/?#]+)\\/+home[\\/]*))(\\?[^#]*)?(#.*)?$"
constantPortion="users//orders/"
members=
<route-group-members>name="users/orders/order"
route=
<route-with-parents>
name="users/orders/order"
elems=${order_id}
matchRegexp="([^\\/?#]+)"
constantPortionRegexp="[^\\/?#]+"
constantPortion=""
constishPrefix=""
nGroups=1
paramGroupNumbers=order_id: 1
tags=[]
methods=[GET]
depth=2
terminal=true
line=7
</route-with-parents>
paramGroupNumbers=order_id: 3, user_id: 2
</route-group-members>
nonparamGroupNumbers[1]
nLevels1
matchRegexp="^(?:(\\/+users\\/+([^\\/?#]+)\\/+orders\\/+([^\\/?#]+)[\\/]*))(\\?[^#]*)?(#.*)?$"
constantPortion="users//profile"
members=
<route-group-members>name="users/profile"
route=
<route-with-parents>
name="users/profile"
elems=${user_id} / 'profile'
matchRegexp="([^\\/?#]+)\\/+profile"
constantPortionRegexp="[^\\/?#]+(\\/)[\\/]*(profile)"
constantPortion="/profile"
constishPrefix=""
nGroups=1
paramGroupNumbers=user_id: 1
tags=[]
methods=[GET]
depth=1
terminal=true
line=5
</route-with-parents>
paramGroupNumbers=user_id: 2
</route-group-members>
nonparamGroupNumbers[1]
nLevels1
matchRegexp="^(?:(\\/+users\\/+([^\\/?#]+)\\/+profile[\\/]*))(\\?[^#]*)?(#.*)?$"
</families>
`

	formatted := debugFormatRouteRegexps(&rrs)
	if strings.TrimSpace(formatted) != strings.TrimSpace(expected) {
		t.Errorf("GetRouteRegexps did not give expected output. Got\n%v\n", formatted)
	}
}

func TestOverlapDetection(t *testing.T) {
	assertOverlap(
		t,
		1, 2,
		""+
			"foo /foo\n"+
			"bar /foo",
	)
	assertOverlap(
		t,
		1, 2,
		""+
			"foo [GET,PUT] /foo\n"+
			"bar [PUT] /foo",
	)
	assertOverlap(
		t,
		1, 2,
		""+
			"foo [GET] /foo\n"+
			"bar [PUT,GET] /foo",
	)
	assertNoOverlap(
		t,
		""+
			"foo [GET] /foo\n"+
			"bar [PUT,POST] /foo",
	)
	assertOverlap(
		t,
		1, 2,
		""+
			"foo /foo\n"+
			"bar /:foo",
	)
	assertNoOverlap(
		t,
		""+
			"foo /foo/\n"+
			"bar /foo!/",
	)
	assertOverlap(
		t,
		1, 2,
		""+
			"foo /foo/\n"+
			"bar /foo",
	)
	assertOverlap(
		t,
		1, 2,
		""+
			"foo /manager/:id/settings\n"+
			"bar /manager/empty/settings",
	)
	assertNoOverlap(
		t,
		""+
			"foo /manager/:#id/settings\n"+
			"bar /manager/empty/settings",
	)
	assertNoOverlap(
		t,
		""+
			"foo /manager/:id/settings\n"+
			"bar /manager/empty/flub",
	)
	assertOverlap(
		t,
		1, 3,
		""+
			"foo /manager\n"+
			"  .\n"+
			"  bar /",
	)
	assertNoOverlap(
		t,
		""+
			"foo /manager!/\n"+
			"  .\n"+
			"  bar /",
	)
	assertNoOverlap(
		t,
		""+
			"foo /manager\n"+
			"  bar /",
	)
	assertNoOverlap(
		t,
		""+
			"foo /\n"+
			"  bar /\n"+
			"    amp /",
	)
	assertOverlap(
		t,
		2, 4,
		""+
			"foo /\n"+
			"  bar /\n"+
			"    .\n"+
			"    amp /",
	)
	assertOverlap(
		t,
		2, 4,
		""+
			"r /\n"+
			"  rr /\n"+
			"    .\n"+
			"    rrr /\n"+
			"      .\n"+
			"      bar /bar\n",
	)
	assertNoOverlap(
		t,
		""+
			"r /\n"+
			"  rr /\n"+
			"    rrr /\n"+
			"      .\n"+
			"      bar /bar\n",
	)
	assertNoOverlap(
		t,
		""+
			"r /\n"+
			"  rr /\n"+
			"    .\n"+
			"    rrr /\n"+
			"      bar /bar\n",
	)
}

func TestDisjoinRegexpComplex(t *testing.T) {
	parents := []*RouteInfo{{Name: "xx", matchRegexp: "PREFIX\\/"}}

	routes := []*RouteWithParents{
		makeSimpleRouteWithParents("foo", "a", parents),
		makeSimpleRouteWithParents("bar$fug$", "b", parents),
		makeSimpleRouteWithParents("amp", "d", []*RouteInfo{}),
		makeSimpleRouteWithParents("one", "e", parents),
		makeSimpleRouteWithParents("two$", "f", []*RouteInfo{}),
		makeSimpleRouteWithParents("three", "g", []*RouteInfo{}),
	}

	testDisjoinRegexp(
		t,
		routes,
		"(?:(((\\/+PREFIX\\/foo[\\/]*)|(\\/+PREFIX\\/bar([^\\/]+)fug([^\\/]+)[\\/]*))|((\\/+amp[\\/]*)|(\\/+PREFIX\\/one[\\/]*)))|(((\\/+two([^\\/]+)[\\/]*)|(\\/+three[\\/]*))))",
		[]map[string]int{{}, {"1": 5, "2": 6}, {}, {}, {"1": 13}, {}},
		[]int{1, 2, 3, 4, 7, 8, 9, 10, 11, 12, 14},
	)
}

func TestRouteMatching(t *testing.T) {
	sl := routeElement{slash, ""}
	glob := routeElement{singleGlob, ""}
	dglob := routeElement{doubleGlob, ""}
	noslash := routeElement{noTrailingSlash, ""}
	c := func(s string) routeElement { return routeElement{constant, s} }
	p := func(s string) routeElement { return routeElement{parameter, s} }
	ip := func(s string) routeElement { return routeElement{integerParameter, s} }
	rp := func(s string) routeElement { return routeElement{restParameter, s} }

	// Initial slash is not included in the raw regexps but is introduced when
	// joining hierarchical routes, so there are no leading slashes in the
	// matching URLs here. Similarly, trailing slashes are handled after
	// hierarchical routes are combined.

	simpleRoute := []routeElement{sl, c("foo")}
	testMatchRoute(t, true, simpleRoute, "foo")
	testMatchRoute(t, false, simpleRoute, "foo/")
	testMatchRoute(t, false, simpleRoute, "foo/")
	testMatchRoute(t, false, simpleRoute, "foo//")
	testMatchRoute(t, false, simpleRoute, "bar")

	simpleRouteNoTrailingSlash := []routeElement{sl, c("foo"), noslash}
	testMatchRoute(t, true, simpleRouteNoTrailingSlash, "foo")
	testMatchRoute(t, false, simpleRouteNoTrailingSlash, "foo/")
	testMatchRoute(t, false, simpleRouteNoTrailingSlash, "foo/")
	testMatchRoute(t, false, simpleRouteNoTrailingSlash, "foo//")
	testMatchRoute(t, false, simpleRouteNoTrailingSlash, "bar")

	withParam := []routeElement{sl, c("foo"), sl, p("myparam"), sl, c("bar")}
	testMatchRoute(t, true, withParam, "foo/myparamvalue/bar", "myparamvalue")
	testMatchRoute(t, false, withParam, "foo//bar")

	withIntParam := []routeElement{sl, c("foo"), sl, ip("myparam"), sl, c("bar")}
	testMatchRoute(t, true, withIntParam, "foo/99/bar", "99")
	testMatchRoute(t, true, withIntParam, "foo/0/bar", "0")
	testMatchRoute(t, true, withIntParam, "foo/00/bar", "00")
	testMatchRoute(t, true, withIntParam, "foo/1234567890123456789/bar", "1234567890123456789")
	testMatchRoute(t, true, withIntParam, "foo/-99/bar", "-99")
	testMatchRoute(t, false, withIntParam, "foo/--99/bar")
	testMatchRoute(t, false, withIntParam, "foo//bar")

	// Weird edge case. Sequential int params can match, even though it would
	// rarely (if ever) be sensible to use them.
	withSequentialIntParams := []routeElement{sl, c("foo"), sl, ip("myparam"), ip("myparam2"), ip("myparam3")}
	testMatchRoute(t, true, withSequentialIntParams, "foo/99-99-99", "99", "-99", "-99")
	// matching is greedy, so without the '-' to indicate the boundary, the first
	// int param is made as long as possible.
	testMatchRoute(t, true, withSequentialIntParams, "foo/999999", "9999", "9", "9")
	// if there are more int params than digits, then no match
	testMatchRoute(t, false, withSequentialIntParams, "foo/99")
	testMatchRoute(t, true, withSequentialIntParams, "foo/999", "9", "9", "9")
	testMatchRoute(t, true, withSequentialIntParams, "foo/-999", "-9", "9", "9")

	withGlob := []routeElement{sl, c("foo"), sl, glob, sl, c("bar")}
	testMatchRoute(t, true, withGlob, "foo/myparamvalue/bar")
	testMatchRoute(t, false, withGlob, "foo//bar")

	// no slash before rest param
	withFinalRestParam := []routeElement{sl, c("foo"), sl, p("myparam"), sl, c("bar"), rp("rest")}
	testMatchRoute(t, true, withFinalRestParam, "foo/myparamvalue/bar/a/lot/of/other/stuff", "myparamvalue", "/a/lot/of/other/stuff")
	testMatchRoute(t, false, withFinalRestParam, "foo/myparamvalue/bar")
	testMatchRoute(t, false, withFinalRestParam, "foo/myparamvalue/bar/")
	testMatchRoute(t, false, withFinalRestParam, "foo/myparamvalue/bar//")

	// with a slash before the rest param
	withFinalRestParam2 := []routeElement{sl, c("foo"), sl, p("myparam"), sl, c("bar"), sl, rp("rest")}
	testMatchRoute(t, true, withFinalRestParam2, "foo/myparamvalue/bar/a/lot/of/other/stuff", "myparamvalue", "a/lot/of/other/stuff")

	// no slash before rest param
	withMiddleRestParam := []routeElement{sl, c("foo"), sl, p("myparam"), sl, c("bar"), rp("rest"), c("term")}
	testMatchRoute(t, true, withMiddleRestParam, "foo/myparamvalue/bar/a/lot/of/other/stuff/term", "myparamvalue", "/a/lot/of/other/stuff/")
	testMatchRoute(t, false, withMiddleRestParam, "foo/myparamvalue/bar")
	testMatchRoute(t, false, withMiddleRestParam, "foo/myparamvalue/bar/")
	testMatchRoute(t, false, withMiddleRestParam, "foo/myparamvalue/bar//")

	// with a slash before the rest param
	withMiddleRestParam2 := []routeElement{sl, c("foo"), sl, p("myparam"), sl, c("bar"), sl, rp("rest"), c("term")}
	testMatchRoute(t, true, withMiddleRestParam2, "foo/myparamvalue/bar/a/lot/of/other/stuff/term", "myparamvalue", "a/lot/of/other/stuff/")
	testMatchRoute(t, false, withMiddleRestParam2, "foo/myparamvalue/bar")
	testMatchRoute(t, false, withMiddleRestParam2, "foo/myparamvalue/bar/")
	testMatchRoute(t, false, withMiddleRestParam2, "foo/myparamvalue/bar//")

	// with a slash after the rest param
	withMiddleRestParam3 := []routeElement{sl, c("foo"), sl, p("myparam"), sl, c("bar"), sl, rp("rest"), sl, c("term")}
	testMatchRoute(t, true, withMiddleRestParam3, "foo/myparamvalue/bar/a/lot/of/other/stuff/term", "myparamvalue", "a/lot/of/other/stuff")

	testMatchRoute(t, true, []routeElement{sl, c("foo"), sl, p("myparam"), sl, c("bar"), dglob}, "foo/myparamvalue/bar/a/lot/of/other/stuff", "myparamvalue")
	testMatchRoute(t, true, []routeElement{sl, c("foo"), sl, p("myparam"), sl, c("bar"), sl, dglob}, "foo/myparamvalue/bar/a/lot/of/other/stuff", "myparamvalue")
	testMatchRoute(t, true, []routeElement{sl, c("foo"), sl, p("myparam"), sl, c("bar"), dglob, c("term")}, "foo/myparamvalue/bar/a/lot/of/other/stuff/term", "myparamvalue")
	testMatchRoute(t, true, []routeElement{sl, c("foo"), sl, p("myparam"), sl, c("bar"), sl, dglob, c("term")}, "foo/myparamvalue/bar/a/lot/of/other/stuff/term", "myparamvalue")
	testMatchRoute(t, true, []routeElement{sl, c("foo"), sl, p("myparam"), sl, c("bar"), sl, dglob, sl, c("term")}, "foo/myparamvalue/bar/a/lot/of/other/stuff/term", "myparamvalue")
}

func TestMatchingSpecs(t *testing.T) {
	if !matchingSpec([]IncludeSpec{}, map[string]struct{}{}, map[string]struct{}{}) {
		t.Errorf("Empty specs should match empty tags")
	}
	if !matchingSpec([]IncludeSpec{}, map[string]struct{}{}, map[string]struct{}{"foo": {}, "bar": {}}) {
		t.Errorf("Empty specs should match non-empty tags")
	}
	if !matchingSpec([]IncludeSpec{{true, "manager/*", ""}}, map[string]struct{}{}, map[string]struct{}{"manager/goo": {}, "manager/bar": {}, "other": {}}) {
		t.Errorf("Simple include match works as expected with glob match")
	}
	if matchingSpec([]IncludeSpec{{false, "manager/*", ""}}, map[string]struct{}{}, map[string]struct{}{"manager/goo": {}, "manager/bar": {}, "other": {}}) {
		t.Errorf("Simple exclude match works as expected with glob match")
	}
	if !matchingSpec([]IncludeSpec{{true, "other", ""}}, map[string]struct{}{}, map[string]struct{}{"manager/goo": {}, "manager/bar": {}, "other": {}}) {
		t.Errorf("Simple include match works as expected with const match")
	}
	if matchingSpec([]IncludeSpec{{false, "other", ""}}, map[string]struct{}{}, map[string]struct{}{"manager/goo": {}, "manager/bar": {}, "other": {}}) {
		t.Errorf("Simple exclude match works as expected with const match")
	}
	if !matchingSpec([]IncludeSpec{{false, "blabble", ""}}, map[string]struct{}{}, map[string]struct{}{"manager/goo": {}, "manager/bar": {}, "other": {}}) {
		t.Errorf("Include by default if we start with exclude")
	}
	if matchingSpec([]IncludeSpec{{true, "blabble", ""}, {false, "blubble", ""}}, map[string]struct{}{}, map[string]struct{}{"manager/goo": {}, "manager/bar": {}, "other": {}}) {
		t.Errorf("Exclude by default if we start with include")
	}
	if !matchingSpec([]IncludeSpec{{true, "manager/*", ""}, {false, "*goo*", ""}, {false, "*bar*", ""}, {true, "*goo*", ""}}, map[string]struct{}{}, map[string]struct{}{"manager/goo": {}, "manager/bar": {}, "other": {}}) {
		t.Errorf("Complex sequence of include...exclude works as expected [1]")
	}
	if matchingSpec([]IncludeSpec{{true, "manager/*", ""}, {false, "*goo*", ""}, {false, "*bar*", ""}}, map[string]struct{}{}, map[string]struct{}{"manager/goo": {}, "manager/bar": {}, "other": {}}) {
		t.Errorf("Complex sequence of include...exclude works as expected [2]")
	}
	if !matchingSpec([]IncludeSpec{{false, "manager/*", ""}, {false, "*oth*", ""}, {true, "*goo*", ""}}, map[string]struct{}{}, map[string]struct{}{"manager/goo": {}, "manager/bar": {}, "other": {}}) {
		t.Errorf("Complex sequence of exclude...include works as expected [1]")
	}
	if matchingSpec([]IncludeSpec{{false, "manager/*", ""}, {false, "*oth*", ""}}, map[string]struct{}{}, map[string]struct{}{"manager/goo": {}, "manager/bar": {}, "other": {}}) {
		t.Errorf("Sequence of excludes works as expected [2]")
	}
	if !matchingSpec([]IncludeSpec{{true, "", "get"}}, map[string]struct{}{"GET": {}, "PUT": {}}, map[string]struct{}{}) {
		t.Errorf("Method matching is case-insensitive [1]")
	}
	if matchingSpec([]IncludeSpec{{false, "", "get"}}, map[string]struct{}{"GET": {}}, map[string]struct{}{}) {
		t.Errorf("Method matching is case-insensitive [2]")
	}
	if matchingSpec([]IncludeSpec{{false, "", "get"}, {false, "", "post"}, {false, "", "put"}}, map[string]struct{}{"GET": {}, "POST": {}, "PUT": {}}, map[string]struct{}{}) {
		t.Errorf("Route should be removed when all its methods are removed")
	}
	if !matchingSpec([]IncludeSpec{{false, "", "get"}, {false, "", "post"}}, map[string]struct{}{"GET": {}, "POST": {}, "PUT": {}}, map[string]struct{}{}) {
		t.Errorf("Route should not be removed if all but one of its methods is removed")
	}
	if !matchingSpec([]IncludeSpec{{false, "", "get"}, {false, "", "post"}, {false, "", "put"}, {true, "", "post"}}, map[string]struct{}{"GET": {}, "POST": {}, "PUT": {}}, map[string]struct{}{}) {
		t.Errorf("Route should not be removed if all its methods are removed and then one is added back")
	}
}

func TestConstantPortion(t *testing.T) {
	type tst struct {
		pattern    string
		expectedCp string
	}

	cases := []tst{
		{"", ""},
		{"/foo/bar", "foo/bar"},
		{"/foo/**/bar", "foo//bar"},
		{"/foo/:**{foo}/bar", "foo//bar"},
		{"/:foo-:bar", "-"},
		{"/:foo--:bar", "--"},
		{"/:foo-:bar-", "--"},
	}

	for _, c := range cases {
		ri := routeToRegexps(parseRoute(c.pattern))
		if ri.constantPortion != c.expectedCp {
			t.Errorf("Expected %v to have constant portion %v, but got %v\n", c.pattern, c.expectedCp, ri.constantPortion)
		}
	}
}

func testMatchRoute(t *testing.T, shouldMatch bool, elems []routeElement, url string, paramValues ...string) {
	cpRegexp, matchRegexp, pgns := getSimpleRouteRegexps(elems)
	mre, err := regexp.Compile(matchRegexp)
	if err != nil {
		t.Errorf("Unexpected error compiling '%v': %v\n", matchRegexp, err)
	}
	m1 := mre.FindStringSubmatch(url)
	m2, err := regexp.MatchString(cpRegexp, url)
	if err != nil {
		t.Errorf("Unexpected error compiling '%v': %v\n", cpRegexp, err)
	}
	if (m1 == nil && m2) || (m1 != nil && !m2) {
		t.Errorf("One matched and the other didn't! %v %v", m1, m2)
	}
	if m2 && !shouldMatch {
		t.Errorf("Expected '%v' not to match '%v', but it did", cpRegexp, url)
	}
	if !m2 && shouldMatch {
		t.Errorf("Expected '%v' to match '%v', but it didn't", cpRegexp, url)
	}

	pvs := make([]string, 0)
	for _, i := range pgns {
		if i >= len(m1) {
			break
		}
		pvs = append(pvs, m1[i])
	}

	if !(len(paramValues) == 0 && len(pvs) == 0) && !reflect.DeepEqual(pvs, paramValues) {
		t.Errorf("Expected params %v, got %v\n", paramValues, pvs)
	}
}

func testDisjoinRegexp(t *testing.T, rs []*RouteWithParents, expected string, paramGroups []map[string]int, nonparamGroups []int) {
	result := disjoinRegexp(rs)
	if result.regex != expected {
		t.Errorf("Disjoining %+v\nExpected %v\nGot %v\n", rs, expected, result.regex)
	}
	if !reflect.DeepEqual(result.paramGroups, paramGroups) {
		t.Errorf("Disjoining %+v\nExpected param groups %v\nGot %v\n", rs, paramGroups, result.paramGroups)
	}
	if !reflect.DeepEqual(result.nonparamGroups, nonparamGroups) {
		t.Errorf("Disjoining %+v\nExpected nonparam groups %+v\nGot %+v\n", rs, nonparamGroups, result.nonparamGroups)
	}
}

func getSimpleRouteRegexps(elems []routeElement) (string, string, []int) {
	ri := routeToRegexps(elems)
	pgns := make([]int, 0)
	for _, n := range ri.paramGroupNumbers {
		pgns = append(pgns, n)
	}
	sort.Ints(pgns)
	cpre := ri.constantPortionRegexp(0)
	return "^" + cpre + "$", "^" + ri.matchRegexp + "$", pgns
}

func makeSimpleRouteWithParents(regexp, name string, parents []*RouteInfo) *RouteWithParents {
	nGroups := 0
	paramGroupNumbers := make(map[string]int)

	var re strings.Builder

	for i := range regexp {
		c := regexp[i]
		if c == '$' {
			nGroups++
			name := fmt.Sprintf("%v", nGroups)
			re.WriteString("([^\\/]+)")
			paramGroupNumbers[name] = nGroups
		} else {
			re.WriteByte(c)
		}
	}

	return &RouteWithParents{
		Parents: parents,
		Route: &RouteInfo{
			Name:              name,
			matchRegexp:       re.String(),
			nGroups:           nGroups,
			paramGroupNumbers: paramGroupNumbers,
		},
	}
}

func assertOverlap(t *testing.T, line1, line2 int, routeFile string) {
	entries, errors := ParseRouteFile(strings.NewReader(routeFile))
	if len(errors) > 0 {
		t.Errorf("Errors parsing route file: %+v\nRoutes:\n%v\n", errors, routeFile)
		return
	}
	_, routeErrors := ProcessRouteFile([][]RouteFileEntry{entries}, []string{""}, "/", func([]RouteWithParents) {})
	if len(routeErrors) != 1 {
		t.Errorf("Expecting to get one route error with an overlap, got %v.\nRoutes:\n%v\n", len(routeErrors), routeFile)
		return
	}
	err := routeErrors[0]
	if err.Kind != OverlappingRoutes {
		t.Errorf("Expecting to get one route error with an overlap, got %v error instead.\nRoutes:\n%v\n", err.Kind, routeFile)
	}
	if line1 != err.Line || line2 != err.OtherLine {
		t.Errorf("Expecting overlap between routes at lines %v and %v, got overlap between routes at lines %v and %v.\nRoutes:\n%v\n", line1, line2, err.Line, err.OtherLine, routeFile)
	}
}

func assertNoOverlap(t *testing.T, routeFile string) {
	entries, errors := ParseRouteFile(strings.NewReader(routeFile))
	if len(errors) > 0 {
		t.Errorf("Errors parsing route file: %+v\nRoutes:\n%v\n", errors, routeFile)
	}
	_, routeErrors := ProcessRouteFile([][]RouteFileEntry{entries}, []string{""}, "/", func([]RouteWithParents) {})
	if len(routeErrors) != 0 {
		t.Errorf("Expecting to get no errors.\nRoutes:\n%v\n", routeFile)
	}
}
