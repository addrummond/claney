package router

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/addrummond/claney/compiler"
)

const routeFile = `
users /users!/
  .
  dot     .
 	home    /:user_id/home
	pro\
	   file /:user_id/profile
	orders  /:user_id/orders
	  order /display/:order_id
managers /managers/ [a tag to \
                     inherit]
  .
  home      /:manager_id/home
  profile   /:manager_id/profile
	stats     [ PUT , POST ] /:manager_id/stats [foo, bar, amp]
	orders    /orders/:user_id/:{o rder_\}\\id}/theorder \
	          [baz]
	test1     [POST] /foo/goo/bar/:maguffin
	test2     foo/bar/:maguffin []
	backslash /routeending\\withbackslash\\
	resty     /foo/blobby/:**rest
	resty     fooo/blobby/:**rest/more
	anotherf  fxoo/blobby/:**rest/more # added for testing porpoises
users /users!/
  foo foo
  another   /x/y/z/k
dupl / # all routes below have the same constant portion
	a /foo.x:#{param}xxxx
	b /foo.xx:#{param}xxx
	c /foo.xxx:#{param}xx
	d /foo.xxxx:#{param}x
	e /foo.xxxxx:#{param}
`

func TestRouter(t *testing.T) {
	testRouter(t, routeFile, false, func(router *Router) {
		assertNoRoute(t, router, "/")
		assertNoRoute(t, router, "")
		assertRoute(t, router, "/users", "users", map[string]string{}, "", "", []string{"GET"}, []string{})
		assertNoRoute(t, router, "/users/")
		assertRoute(t, router, "/users/.", "users/dot", map[string]string{}, "", "", []string{"GET"}, []string{})
		assertRoute(t, router, "/users/123/home", "users/home", map[string]string{"user_id": "123"}, "", "", []string{"GET"}, []string{})
		assertRoute(t, router, "/users/123//profile/", "users/profile", map[string]string{"user_id": "123"}, "", "", []string{"GET"}, []string{})
		assertNoRoute(t, router, "/users/123/orders")
		assertRoute(t, router, "/users/123/orders/display/456", "users/orders/order", map[string]string{"user_id": "123", "order_id": "456"}, "", "", []string{"GET"}, []string{})

		assertNoRoute(t, router, "/managers")
		assertRoute(t, router, "/managers/", "managers", map[string]string{}, "", "", []string{"GET"}, []string{"a tag to inherit"})
		assertRoute(t, router, "/managers/123/home//", "managers/home", map[string]string{"manager_id": "123"}, "", "", []string{"GET"}, []string{"a tag to inherit"})
		assertRoute(t, router, "/MANAGERS/123/profile", "managers/profile", map[string]string{"manager_id": "123"}, "", "", []string{"GET"}, []string{"a tag to inherit"})
		assertRoute(t, router, "/managers/123/stats", "managers/stats", map[string]string{"manager_id": "123"}, "", "", []string{"POST", "PUT"}, []string{"a tag to inherit", "amp", "bar", "foo"})
		assertRoute(t, router, "/managers/orders/123/456/theorder", "managers/orders", map[string]string{"user_id": "123", "o rder_}\\id": "456"}, "", "", []string{"GET"}, []string{"a tag to inherit", "baz"})
		assertRoute(t, router, "/managers/foo//goo/bar/123", "managers/test1", map[string]string{"maguffin": "123"}, "", "", []string{"POST"}, []string{"a tag to inherit"})
		assertRoute(t, router, "/managers/Foo/bar/123", "managers/test2", map[string]string{"maguffin": "123"}, "", "", []string{"GET"}, []string{"a tag to inherit"})
		assertRoute(t, router, "/mAnAgeRs/routeending\\withbackslash\\", "managers/backslash", map[string]string{}, "", "", []string{"GET"}, []string{"a tag to inherit"})
		assertRoute(t, router, "/managers/foo/blobby/some/other/stuff/bar", "managers/resty", map[string]string{"rest": "some/other/stuff/bar"}, "", "", []string{"GET"}, []string{"a tag to inherit"})
		assertRoute(t, router, "/managers/fooo/blobby/some/other/stuff/more", "managers/resty", map[string]string{"rest": "some/other/stuff"}, "", "", []string{"GET"}, []string{"a tag to inherit"})
		assertNoRoute(t, router, "/managers/foo/blobby")
		assertNoRoute(t, router, "/managers/foo/blobby/")
		assertNoRoute(t, router, "/managers/foo/blobby//")
		assertNoRoute(t, router, "/managers/fooo/blobby/more")
		assertNoRoute(t, router, "/managers/fooo/blobby//more")

		assertRoute(t, router, "/foo.x123xxxx", "dupl/a", map[string]string{"param": "123"}, "", "", []string{"GET"}, []string{})
		assertRoute(t, router, "/foo.xx123xxx", "dupl/b", map[string]string{"param": "123"}, "", "", []string{"GET"}, []string{})
		assertRoute(t, router, "/foo.xxx123xx", "dupl/c", map[string]string{"param": "123"}, "", "", []string{"GET"}, []string{})
		assertRoute(t, router, "/foo.xxxx123x", "dupl/d", map[string]string{"param": "123"}, "", "", []string{"GET"}, []string{})
		assertRoute(t, router, "/foo.xxxxx123", "dupl/e", map[string]string{"param": "123"}, "", "", []string{"GET"}, []string{})

		assertRoute(t, router, "/managers/123/profile?with=aquery&strinG=BAR", "managers/profile", map[string]string{"manager_id": "123"}, "?with=aquery&strinG=BAR", "", []string{"GET"}, []string{"a tag to inherit"})
		assertRoute(t, router, "/managers/foo/bar/123?with=aquery&string=foo", "managers/test2", map[string]string{"maguffin": "123"}, "?with=aquery&string=foo", "", []string{"GET"}, []string{"a tag to inherit"})
		assertRoute(t, router, "/foo.xxxx123x#foo?q=a&boo=c", "dupl/d", map[string]string{"param": "123"}, "", "#foo?q=a&boo=c", []string{"GET"}, []string{})
		assertRoute(t, router, "/foo.xxxxx123?q=a#foo", "dupl/e", map[string]string{"param": "123"}, "?q=a", "#foo", []string{"GET"}, []string{})

		assertRoute(t, router, "/MaNaGeRs/?foO=BaR", "managers", map[string]string{}, "?foO=BaR", "", []string{"GET"}, []string{"a tag to inherit"})
	})
}

func TestRouterFinickySlashStuff1(t *testing.T) {
	const routeFile = `
	noslash /foo!/
    withslash /
	`

	testRouter(t, routeFile, false, func(router *Router) {
		assertNoRoute(t, router, "/foo")
		assertRoute(t, router, "/foo/", "noslash/withslash", map[string]string{}, "", "", []string{"GET"}, []string{})
	})
}

func TestRouterFinickySlashStuff2(t *testing.T) {
	const routeFile = `
  noslash /foo!/
    .
    withslash /
  `

	testRouter(t, routeFile, false, func(router *Router) {
		assertRoute(t, router, "/foo", "noslash", map[string]string{}, "", "", []string{"GET"}, []string{})
		assertRoute(t, router, "/foo/", "noslash/withslash", map[string]string{}, "", "", []string{"GET"}, []string{})
	})
}

func TestRouterFinickySlashStuff3(t *testing.T) {
	const routeFile = `
trail   /foo/
notrail /foo!/
  `

	testRouter(t, routeFile, false, func(router *Router) {
		assertRoute(t, router, "/foo/", "trail", map[string]string{}, "", "", []string{"GET"}, []string{})
		assertRoute(t, router, "/foo", "notrail", map[string]string{}, "", "", []string{"GET"}, []string{})
	})
}

func TestRouterFinickySlashStuff4(t *testing.T) {
	const routeFile = `
r /
  rr /
	  rrr /
		  .
			bar /bar
  `

	testRouter(t, routeFile, false, func(router *Router) {
		assertRoute(t, router, "/", "r/rr/rrr", map[string]string{}, "", "", []string{"GET"}, []string{})
		assertRoute(t, router, "/bar", "r/rr/bar", map[string]string{}, "", "", []string{"GET"}, []string{})
	})
}

func TestRouterCaseSensitivity(t *testing.T) {
	const routeFile = "r /Foo/Bar"

	testRouter(t, routeFile, true, func(router *Router) {
		assertNoRoute(t, router, "/foo/bar")
		assertNoRoute(t, router, "/FOO/BAR")
		assertRoute(t, router, "/Foo/Bar", "r", map[string]string{}, "", "", []string{"GET"}, []string{})
	})
}

func TestRouterOptimization(t *testing.T) {
	const routeFile = `
foo11 /11
foo12 /12
foo13 /13
foo14 /14
foo15 /15
foo16 /16
foo17 /17
foo171 /171
foo172 /172
foo173 /173
foo174 /174
foo175 /175
foo21 /21
foo22 /22
foo23 /23
foo24 /24
foo25 /25
foo26 /26
foo27 /27
foo31 /31
foo32 /32
foo33 /33
foo34 /34
foo35 /35
foo36 /36
foo37 /37
  `

	testRouter(t, routeFile, false, func(router *Router) {
		for i := 1; i <= 3; i++ {
			for j := 1; j <= 7; j++ {
				assertRoute(t, router, fmt.Sprintf("/%v%v", i, j), fmt.Sprintf("foo%v%v", i, j), map[string]string{}, "", "", []string{"GET"}, []string{})
			}
		}
		assertRoute(t, router, "/171", "foo171", map[string]string{}, "", "", []string{"GET"}, []string{})
		assertRoute(t, router, "/172", "foo172", map[string]string{}, "", "", []string{"GET"}, []string{})
		assertRoute(t, router, "/173", "foo173", map[string]string{}, "", "", []string{"GET"}, []string{})
		assertRoute(t, router, "/174", "foo174", map[string]string{}, "", "", []string{"GET"}, []string{})
		assertRoute(t, router, "/175", "foo175", map[string]string{}, "", "", []string{"GET"}, []string{})
		for i := 4; i <= 9; i++ {
			for j := 8; j <= 9; j++ {
				assertNoRoute(t, router, fmt.Sprintf("%v%v", i, j))
			}
		}
		for i := 1; i <= 3; i++ {
			for j := 1; j <= 7; j++ {
				assertNoRoute(t, router, fmt.Sprintf("%v%v1", i, j))
			}
		}
	})
}

func TestRouterBinarySearch(t *testing.T) {
	for n := 1; n < 100; n++ {
		bsRoute := makeBSRouteFile(n)

		testRouter(t, bsRoute, false, func(router *Router) {
			for i := 1; i < n; i++ {
				assertRoute(t, router, makeBSRoute(i, n, "123"), fmt.Sprintf("route%v", i), map[string]string{"param": "123"}, "", "", []string{"GET"}, []string{})
			}
		})
	}
}

func TestNormalizeUrl(t *testing.T) {
	type tst struct {
		from, to string
	}

	cases := []tst{
		{"", ""},
		{"/foo/bar?boo=blab", "/foo/bar?boo=blab"},
		{"/foo/bar?boo=BLAb", "/foo/bar?boo=BLAb"},
		{"/fOo/bAr?foo=BLAB", "/foo/bar?foo=BLAB"},
		{"/fOo/bAr?foo=BLAB?blah=fUg", "/foo/bar?foo=BLAB?blah=fUg"},
		{"/foo/bar", "/foo/bar"},
		{"/fOo/bAr", "/foo/bar"},
	}

	for _, c := range cases {
		out := normalizeUrl(c.from)
		if out != c.to {
			t.Errorf("Expected %v -> %v, got %v\n", c.from, c.to, out)
		}
	}
}

// make route file with pattern of routes used to test binary search part of routing algo
func makeBSRouteFile(n int) string {
	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteString(fmt.Sprintf("route%v ", i))
		sb.WriteString("/foo.")
		for j := 0; j < i; j++ {
			sb.WriteRune('x')
		}
		sb.WriteString(":#{param}")
		for j := 0; j < n-i; j++ {
			sb.WriteRune('x')
		}
		sb.WriteRune('\n')
	}
	return sb.String()
}

// make url with pattern used to test binary search part of routing algo
func makeBSRoute(i, n int, param string) string {
	var sb strings.Builder
	sb.WriteString("/foo.")
	for j := 0; j < i; j++ {
		sb.WriteRune('x')
	}
	sb.WriteString(param)
	for j := 0; j < n-i; j++ {
		sb.WriteRune('x')
	}

	return sb.String()
}

func testRouter(t *testing.T, routeFile string, caseSensitive bool, callback func(*Router)) {
	casePolicy := compiler.DisallowUpperCase
	if caseSensitive {
		casePolicy = compiler.AllowUpperCase
	}

	entries, errors := compiler.ParseRouteFile(strings.NewReader(routeFile), casePolicy)
	if len(errors) > 0 {
		t.Errorf("Errors parsing route file: %+v\n", errors)
	}
	routes, routeErrors := compiler.ProcessRouteFiles([][]compiler.RouteFileEntry{entries}, []string{""}, "/")
	if len(routeErrors) > 0 {
		t.Errorf("Errors processing route file: %+v\n", routeErrors)
	}
	rrs := compiler.GetRouteRegexps(routes, nil)
	routesJson, _ := compiler.RouteRegexpsToJSON(&rrs, nil)
	//fmt.Printf("JS %v\n", string(routesJson))

	router, err := MakeRouter(routesJson, caseSensitive)
	if err != nil {
		t.Errorf("%v\n", err)
	}

	callback(&router)
}

func assertNoRoute(t *testing.T, router *Router, url string) {
	_, ok := Route(router, url)
	if ok {
		t.Errorf("Expected %v not to be found\n", url)
		return
	}
}

func assertRoute(t *testing.T, router *Router, url, expectedName string, expectedParams map[string]string, expectedQuery string, expectedAnchor string, expectedMethods []string, expectedTags []string) {
	routeResult, ok := Route(router, url)
	if !ok {
		t.Errorf("Expected %v to be found\n", url)
		return
	}

	if routeResult.Name != expectedName {
		t.Errorf("Expected to resolve to '%v', got '%v'\n", expectedName, routeResult.Name)
	}

	if !reflect.DeepEqual(routeResult.Params, expectedParams) {
		t.Errorf("Expected params: %+v\nGot params: %+v\n", expectedParams, routeResult.Params)
	}

	if routeResult.Query != expectedQuery {
		t.Errorf("Expected query: %v\nGot query: %v\n", expectedQuery, routeResult.Query)
	}

	if routeResult.Anchor != expectedAnchor {
		t.Errorf("Expected anchor: %v\nGot anchor: %v\n", expectedAnchor, routeResult.Anchor)
	}

	if !reflect.DeepEqual(routeResult.Methods, expectedMethods) {
		t.Errorf("Expected methods: %+v\nGot methods: %+v\n", expectedMethods, routeResult.Methods)
	}

	if !reflect.DeepEqual(routeResult.Tags, expectedTags) {
		t.Errorf("Expected tags: %+v\nGot tags: %+v\n", expectedTags, routeResult.Tags)
	}
}

func benchmarkRouterSimpleRoutes(b *testing.B, nRoutes int) {
	var sb strings.Builder
	for i := 0; i < nRoutes; i++ {
		sb.WriteString(fmt.Sprintf("%vfoo /%vfoo\n", i, i))
	}
	routeFile := sb.String()
	entries, errors := compiler.ParseRouteFile(strings.NewReader(routeFile), compiler.DisallowUpperCase)
	if len(errors) > 0 {
		b.Errorf("Errors parsing route file: %+v\n", errors)
	}
	routes, routeErrors := compiler.ProcessRouteFiles([][]compiler.RouteFileEntry{entries}, []string{""}, "/")
	if len(routeErrors) > 0 {
		b.Errorf("Errors processing route file: %+v\n", routeErrors)
	}
	rrs := compiler.GetRouteRegexps(routes, nil)
	routesJson, _ := compiler.RouteRegexpsToJSON(&rrs, nil)
	// use the line below to generate the files in js/bench_data, if they need to be updated.
	//os.WriteFile(fmt.Sprintf("routes%v", nRoutes), []byte(routesJson), 0)
	router, err := MakeRouter(routesJson, false)
	if err != nil {
		b.Errorf("%v\n", err)
	}

	urlsToTest := make([]string, 10)
	for i := 0; i < 10; i++ {
		urlsToTest[i] = fmt.Sprintf("/%vfoo", i*(nRoutes/10))
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		Route(&router, urlsToTest[b.N%10])
	}
}

func BenchmarkRouterSimpleRoutes10(b *testing.B) {
	benchmarkRouterSimpleRoutes(b, 10)
}

func BenchmarkRouterSimpleRoutes100(b *testing.B) {
	benchmarkRouterSimpleRoutes(b, 100)
}

func BenchmarkRouterSimpleRoutes1000(b *testing.B) {
	benchmarkRouterSimpleRoutes(b, 1000)
}

func BenchmarkRouterSimpleRoutes10000(b *testing.B) {
	benchmarkRouterSimpleRoutes(b, 10000)
}
