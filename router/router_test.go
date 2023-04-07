package router

import (
	"fmt"
	"os"
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
	testRouter(t, routeFile, func(router *Router) {
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
		assertRoute(t, router, "/managers/123/profile", "managers/profile", map[string]string{"manager_id": "123"}, "", "", []string{"GET"}, []string{"a tag to inherit"})
		assertRoute(t, router, "/managers/123/stats", "managers/stats", map[string]string{"manager_id": "123"}, "", "", []string{"POST", "PUT"}, []string{"a tag to inherit", "amp", "bar", "foo"})
		assertRoute(t, router, "/managers/orders/123/456/theorder", "managers/orders", map[string]string{"user_id": "123", "o rder_}\\id": "456"}, "", "", []string{"GET"}, []string{"a tag to inherit", "baz"})
		assertRoute(t, router, "/managers/foo//goo/bar/123", "managers/test1", map[string]string{"maguffin": "123"}, "", "", []string{"POST"}, []string{"a tag to inherit"})
		assertRoute(t, router, "/managers/foo/bar/123", "managers/test2", map[string]string{"maguffin": "123"}, "", "", []string{"GET"}, []string{"a tag to inherit"})
		assertRoute(t, router, "/managers/routeending\\withbackslash\\", "managers/backslash", map[string]string{}, "", "", []string{"GET"}, []string{"a tag to inherit"})
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

		assertRoute(t, router, "/managers/123/profile?with=aquery&string=bar", "managers/profile", map[string]string{"manager_id": "123"}, "?with=aquery&string=bar", "", []string{"GET"}, []string{"a tag to inherit"})
		assertRoute(t, router, "/managers/foo/bar/123?with=aquery&string=foo", "managers/test2", map[string]string{"maguffin": "123"}, "?with=aquery&string=foo", "", []string{"GET"}, []string{"a tag to inherit"})
		assertRoute(t, router, "/foo.xxxx123x#foo?q=a&boo=c", "dupl/d", map[string]string{"param": "123"}, "", "#foo?q=a&boo=c", []string{"GET"}, []string{})
		assertRoute(t, router, "/foo.xxxxx123?q=a#foo", "dupl/e", map[string]string{"param": "123"}, "?q=a", "#foo", []string{"GET"}, []string{})
	})
}

func TestRouterFinickySlashStuff1(t *testing.T) {
	const routeFile = `
	noslash /foo!/
    withslash /
	`

	testRouter(t, routeFile, func(router *Router) {
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

	testRouter(t, routeFile, func(router *Router) {
		assertRoute(t, router, "/foo", "noslash", map[string]string{}, "", "", []string{"GET"}, []string{})
		assertRoute(t, router, "/foo/", "noslash/withslash", map[string]string{}, "", "", []string{"GET"}, []string{})
	})
}

func TestRouterFinickySlashStuff3(t *testing.T) {
	const routeFile = `
trail   /foo/
notrail /foo!/
  `

	testRouter(t, routeFile, func(router *Router) {
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

	testRouter(t, routeFile, func(router *Router) {
		assertRoute(t, router, "/", "r/rr/rrr", map[string]string{}, "", "", []string{"GET"}, []string{})
		assertRoute(t, router, "/bar", "r/rr/bar", map[string]string{}, "", "", []string{"GET"}, []string{})
	})
}

func TestRouterBinarySearch(t *testing.T) {
	for n := 1; n < 100; n++ {
		bsRoute := makeBSRouteFile(n)

		testRouter(t, bsRoute, func(router *Router) {
			for i := 1; i < n; i++ {
				assertRoute(t, router, makeBSRoute(i, n, "123"), fmt.Sprintf("route%v", i), map[string]string{"param": "123"}, "", "", []string{"GET"}, []string{})
			}
		})
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

func testRouter(t *testing.T, routeFile string, callback func(*Router)) {
	entries, errors := compiler.ParseRouteFile(strings.NewReader(routeFile))
	if len(errors) > 0 {
		t.Errorf("Errors parsing route file: %+v\n", errors)
	}
	routes, routeErrors := compiler.ProcessRouteFile([][]compiler.RouteFileEntry{entries}, []string{""}, "/", func([]compiler.RouteWithParents) {})
	if len(routeErrors) > 0 {
		t.Errorf("Errors processing route file: %+v\n", routeErrors)
	}
	rrs := compiler.GetRouteRegexps(routes)
	routesJson, _ := compiler.RouteRegexpsToJSON(&rrs, []compiler.IncludeSpec{})
	//fmt.Printf("JS %v\n", string(routesJson))

	router, err := MakeRouter(routesJson)
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

	if routeResult.name != expectedName {
		t.Errorf("Expected to resolve to '%v', got '%v'\n", expectedName, routeResult.name)
	}

	if !reflect.DeepEqual(routeResult.params, expectedParams) {
		t.Errorf("Expected params: %+v\nGot params: %+v\n", expectedParams, routeResult.params)
	}

	if routeResult.query != expectedQuery {
		t.Errorf("Expected query: %v\nGot query: %v\n", expectedQuery, routeResult.query)
	}

	if routeResult.anchor != expectedAnchor {
		t.Errorf("Expected anchor: %v\nGot anchor: %v\n", expectedAnchor, routeResult.anchor)
	}

	if !reflect.DeepEqual(routeResult.methods, expectedMethods) {
		t.Errorf("Expected methods: %+v\nGot methods: %+v\n", expectedMethods, routeResult.methods)
	}

	if !reflect.DeepEqual(routeResult.tags, expectedTags) {
		t.Errorf("Expected tags: %+v\nGot tags: %+v\n", expectedTags, routeResult.tags)
	}
}

func benchmarkRouterSimpleRoutes(b *testing.B, nRoutes int) {
	var sb strings.Builder
	for i := 0; i < nRoutes; i++ {
		sb.WriteString(fmt.Sprintf("%vfoo /%vfoo\n", i, i))
	}
	routeFile := sb.String()
	entries, errors := compiler.ParseRouteFile(strings.NewReader(routeFile))
	if len(errors) > 0 {
		b.Errorf("Errors parsing route file: %+v\n", errors)
	}
	routes, routeErrors := compiler.ProcessRouteFile([][]compiler.RouteFileEntry{entries}, []string{""}, "/", func([]compiler.RouteWithParents) {})
	if len(routeErrors) > 0 {
		b.Errorf("Errors processing route file: %+v\n", routeErrors)
	}
	rrs := compiler.GetRouteRegexps(routes)
	routesJson, _ := compiler.RouteRegexpsToJSON(&rrs, []compiler.IncludeSpec{})
	os.WriteFile(fmt.Sprintf("routes%v", nRoutes), []byte(routesJson), 0)
	router, err := MakeRouter(routesJson)
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
