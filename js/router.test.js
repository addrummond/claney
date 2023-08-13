import { Router, normalizeUrl } from './router';

describe('router', () => {
  // Keep in sync with value of 'routesJson' var in TestRouter in router/router_test.go
  const ROUTE_INFO = {"constantPortionNGroups":77,"constantPortionRegexp":"^(?:(?:\\/+(users))(?:|(?:(\\/)\\/*(\\.)\\/*)|(?:(\\/)\\/*[^\\/?#]+(\\/)\\/*(home)\\/*)|(?:(\\/)\\/*[^\\/?#]+(\\/)\\/*(profile)\\/*)|(?:(\\/)\\/*[^\\/?#]+(\\/)\\/*(orders))(?:(?:(\\/)\\/*(display)(\\/)\\/*[^\\/?#]+\\/*)))|(?:\\/+(managers))(?:\\/+|(?:(\\/)\\/*[^\\/?#]+(\\/)\\/*(home)\\/*)|(?:(\\/)\\/*[^\\/?#]+(\\/)\\/*(profile)\\/*)|(?:(\\/)\\/*[^\\/?#]+(\\/)\\/*(stats)\\/*)|(?:(\\/)\\/*(orders)(\\/)\\/*[^\\/?#]+(\\/)\\/*[^\\/?#]+(\\/)\\/*(theorder)\\/*)|(?:(\\/)\\/*(foo)(\\/)\\/*(goo)(\\/)\\/*(bar)(\\/)\\/*[^\\/?#]+\\/*)|(?:(\\/)\\/*(foo)(\\/)\\/*(bar)(\\/)\\/*[^\\/?#]+\\/*)|(?:(\\/)\\/*(routeending\\\\withbackslash\\\\)\\/*)|(?:(\\/)\\/*(foo)(\\/)\\/*(blobby)(\\/)\\/*\\/*[^\\/?#][^?#]*\\/*)|(?:(\\/)\\/*(fooo)(\\/)\\/*(blobby)(\\/)\\/*\\/*[^\\/?#][^?#]*?(\\/)\\/*(more)\\/*))|(?:\\/+(users))(?:(?:(\\/)\\/*(foo)\\/*)|(?:(\\/)\\/*(x)(\\/)\\/*(y)(\\/)\\/*(z)(\\/)\\/*(k)\\/*))|(?:\\/+)(?:(?:(foo\\.x)-?[0-9]+(xxxx)\\/*)|(?:(foo\\.xx)-?[0-9]+(xxx)\\/*)|(?:(foo\\.xxx)-?[0-9]+(xx)\\/*)|(?:(foo\\.xxxx)-?[0-9]+(x)\\/*)|(?:(foo\\.xxxxx)-?[0-9]+\\/*)))(?:\\?[^#]*)?(?:#.*)?$","families":{"foo.xxxxx":{"matchRegexp":"^(?:(((\\/+foo\\.x(-?[0-9]+)xxxx\\/*)|(\\/+foo\\.xx(-?[0-9]+)xxx\\/*))|((\\/+foo\\.xxx(-?[0-9]+)xx\\/*)|(\\/+foo\\.xxxx(-?[0-9]+)x\\/*)))|(((\\/+foo\\.xxxxx(-?[0-9]+)\\/*))))(\\?[^#]*)?(#.*)?$","nLevels":3,"nonparamGroupNumbers":[1,2,3,5,7,8,10,12,13,14],"members":[{"name":"dupl/a","paramGroupNumbers":{"param":4},"tags":[],"methods":["GET"]},{"name":"dupl/b","paramGroupNumbers":{"param":6},"tags":[],"methods":["GET"]},{"name":"dupl/c","paramGroupNumbers":{"param":9},"tags":[],"methods":["GET"]},{"name":"dupl/d","paramGroupNumbers":{"param":11},"tags":[],"methods":["GET"]},{"name":"dupl/e","paramGroupNumbers":{"param":15},"tags":[],"methods":["GET"]}]},"managers":{"matchRegexp":"^(?:(\\/+managers\\/+))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"managers","paramGroupNumbers":{},"tags":["a tag to inherit"],"methods":["GET"]}]},"managers//home":{"matchRegexp":"^(?:(\\/+managers\\/+([^\\/?#]+)\\/+home\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"managers/home","paramGroupNumbers":{"manager_id":2},"tags":["a tag to inherit"],"methods":["GET"]}]},"managers//profile":{"matchRegexp":"^(?:(\\/+managers\\/+([^\\/?#]+)\\/+profile\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"managers/profile","paramGroupNumbers":{"manager_id":2},"tags":["a tag to inherit"],"methods":["GET"]}]},"managers//stats":{"matchRegexp":"^(?:(\\/+managers\\/+([^\\/?#]+)\\/+stats\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"managers/stats","paramGroupNumbers":{"manager_id":2},"tags":["a tag to inherit","amp","bar","foo"],"methods":["POST","PUT"]}]},"managers/foo/bar/":{"matchRegexp":"^(?:(\\/+managers\\/+foo\\/+bar\\/+([^\\/?#]+)\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"managers/test2","paramGroupNumbers":{"maguffin":2},"tags":["a tag to inherit"],"methods":["GET"]}]},"managers/foo/blobby/":{"matchRegexp":"^(?:(\\/+managers\\/+foo\\/+blobby\\/+(\\/*[^\\/?#][^?#]*)\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"managers/resty","paramGroupNumbers":{"rest":2},"tags":["a tag to inherit"],"methods":["GET"]}]},"managers/foo/goo/bar/":{"matchRegexp":"^(?:(\\/+managers\\/+foo\\/+goo\\/+bar\\/+([^\\/?#]+)\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"managers/test1","paramGroupNumbers":{"maguffin":2},"tags":["a tag to inherit"],"methods":["POST"]}]},"managers/fooo/blobby//more":{"matchRegexp":"^(?:(\\/+managers\\/+fooo\\/+blobby\\/+(\\/*[^\\/?#][^?#]*?)\\/+more\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"managers/resty","paramGroupNumbers":{"rest":2},"tags":["a tag to inherit"],"methods":["GET"]}]},"managers/orders///theorder":{"matchRegexp":"^(?:(\\/+managers\\/+orders\\/+([^\\/?#]+)\\/+([^\\/?#]+)\\/+theorder\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"managers/orders","paramGroupNumbers":{"user_id":2,"o rder_}\\id":3},"tags":["a tag to inherit","baz"],"methods":["GET"]}]},"managers/routeending\\withbackslash\\":{"matchRegexp":"^(?:(\\/+managers\\/+routeending\\\\withbackslash\\\\\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"managers/backslash","paramGroupNumbers":{},"tags":["a tag to inherit"],"methods":["GET"]}]},"users":{"matchRegexp":"^(?:(\\/+users))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"users","paramGroupNumbers":{},"tags":[],"methods":["GET"]}]},"users/.":{"matchRegexp":"^(?:(\\/+users\\/+\\.\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"users/dot","paramGroupNumbers":{},"tags":[],"methods":["GET"]}]},"users//home":{"matchRegexp":"^(?:(\\/+users\\/+([^\\/?#]+)\\/+home\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"users/home","paramGroupNumbers":{"user_id":2},"tags":[],"methods":["GET"]}]},"users//orders/display/":{"matchRegexp":"^(?:(\\/+users\\/+([^\\/?#]+)\\/+orders\\/+display\\/+([^\\/?#]+)\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"users/orders/order","paramGroupNumbers":{"user_id":2,"order_id":3},"tags":[],"methods":["GET"]}]},"users//profile":{"matchRegexp":"^(?:(\\/+users\\/+([^\\/?#]+)\\/+profile\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"users/profile","paramGroupNumbers":{"user_id":2},"tags":[],"methods":["GET"]}]},"users/foo":{"matchRegexp":"^(?:(\\/+users\\/+foo\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"users/foo","paramGroupNumbers":{},"tags":[],"methods":["GET"]}]},"users/x/y/z/k":{"matchRegexp":"^(?:(\\/+users\\/+x\\/+y\\/+z\\/+k\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"users/another","paramGroupNumbers":{},"tags":[],"methods":["GET"]}]}}};
  const router = new Router(ROUTE_INFO);

  test('yields expected results for various routes', () => {
    expect(router.route("/")).toEqual(null);
    expect(router.route("")).toEqual(null);
    expect(router.route("/users")).toEqual({
      name: "users",
      methods: ["GET"],
      params: {},
      query: "",
      anchor: "",
      tags: []
    });
    expect(router.route("/users/.")).toEqual({
      name: "users/dot",
      methods: ["GET"],
      params: {},
      query: "",
      anchor: "",
      tags: []
    });
    expect(router.route("/users/123/home//")).toEqual({
      name: "users/home",
      methods: ["GET"],
      params: {user_id: "123"},
      query: "",
      anchor: "",
      tags: []
    });
    expect(router.route("/users//123/profile")).toEqual({
      name: "users/profile",
      methods: ["GET"],
      params: {user_id: "123"},
      query: "",
      anchor: "",
      tags: []
    });
    expect(router.route("/users/123/orders")).toEqual(null);
    expect(router.route("/users/123/orders/display/456")).toEqual({
      name: "users/orders/order",
      methods: ["GET"],
      params: {user_id: "123", order_id: "456"},
      query: "",
      anchor: "",
      tags: []
    });

    expect(router.route("managers")).toEqual(null);
    expect(router.route("/managers//123/home/")).toEqual({
      name: "managers/home",
      methods: ["GET"],
      params: {manager_id: "123"},
      query: "",
      anchor: "",
      tags: ["a tag to inherit"]
    });
    expect(router.route("/managers/123/stats///")).toEqual({
      name: "managers/stats",
      methods: ["POST", "PUT"],
      params: {manager_id: "123"},
      query: "",
      anchor: "",
      tags: ["a tag to inherit", "amp", "bar", "foo"]
    });
    expect(router.route("/managers/orders/123/456/theorder")).toEqual({
      name: "managers/orders",
      methods: ["GET"],
      params: {user_id: "123", "o rder_}\\id": "456"},
      query: "",
      anchor: "",
      tags: ["a tag to inherit", "baz"]
    });
    expect(router.route("/managers/foo/goo/bar/123")).toEqual({
      name: "managers/test1",
      methods: ["POST"],
      params: {maguffin: "123"},
      query: "",
      anchor: "",
      tags: ["a tag to inherit"]
    });
    expect(router.route("/managers/foo/bar/123")).toEqual({
      name: "managers/test2",
      methods: ["GET"],
      params: {maguffin: "123"},
      query: "",
      anchor: "",
      tags: ["a tag to inherit"]
    });
    expect(router.route("/managers/routeending\\withbackslash\\")).toEqual({
      name: "managers/backslash",
      methods: ["GET"],
      params: {},
      query: "",
      anchor: "",
      tags: ["a tag to inherit"]
    });

    expect(router.route("/foo.x123xxxx")).toEqual({
      name: "dupl/a",
      params: {"param": "123"},
      methods: ["GET"],
      query: "",
      anchor: "",
      tags: []
    });
    expect(router.route("/foo.xx123xxx#foo")).toEqual({
      name: "dupl/b",
      methods: ["GET"],
      params: {"param": "123"},
      query: "",
      anchor: "#foo",
      tags: []
    });
    expect(router.route("/foo.xxx123xx")).toEqual({
      name: "dupl/c",
      methods: ["GET"],
      params: {"param": "123"},
      query: "",
      anchor: "",
      tags: []
    });
    expect(router.route("/foo.xxxx123x")).toEqual({
      name: "dupl/d",
      methods: ["GET"],
      params: {"param": "123"},
      query: "",
      anchor: "",
      tags: []
    });
    expect(router.route("/foo.xxxxx123")).toEqual({
      name: "dupl/e",
      methods: ["GET"],
      params: {"param": "123"},
      query: "",
      anchor: "",
      tags: []
    });

    expect(router.route("/managers/123/profile?with=aquery&string=bar")).toEqual({
      name: "managers/profile",
      methods: ["GET"],
      params: {"manager_id": "123"},
      query: "?with=aquery&string=bar",
      anchor: "",
      tags: ["a tag to inherit"]
    });
    expect(router.route("/managers/foo/bar/123?with=aquery&string=foo")).toEqual({
      name: "managers/test2",
      methods: ["GET"],
      params: {"maguffin": "123"},
      query: "?with=aquery&string=foo",
      anchor: "",
      tags: ["a tag to inherit"]
    });
    expect(router.route("/foo.xxxx123x#foo?q=a&boo=c")).toEqual({
      name: "dupl/d",
      methods: ["GET"],
      params: {"param": "123"},
      query: "",
      anchor: "#foo?q=a&boo=c",
      tags: []
    });
    expect(router.route("/foo.xxxxx123?q=a#foo")).toEqual({
      name: "dupl/e",
      methods: ["GET"],
      params: {"param": "123"},
      query: "?q=a",
      anchor: "#foo",
      tags: []
    });

    expect(router.route("/MaNaGeRs/123/Home/?foO=BaR")).toEqual({
      name: "managers/home",
      methods: ["GET"],
      params: {manager_id: "123"},
      query: "?foO=BaR",
      anchor: "",
      tags: ["a tag to inherit"]
    });
  });
})

describe('case sensitive router', () => {
  // Simple router that defines a single route 'r' with the URL "/Foo/Bar"
  const ROUTE_INFO = {"constantPortionNGroups":3,"constantPortionRegexp":"^(?:\\/+(?:(Foo)(\\/)\\/*(Bar)\\/*))(?:\\?[^#]*)?(?:#.*)?$","families":{"Foo/Bar":{"matchRegexp":"^(?:(\\/+Foo\\/+Bar\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"r","paramGroupNumbers":{},"tags":[],"methods":["GET"]}]}}};
  const router = new Router(ROUTE_INFO, true);

  test("is case sensitive", () => {
    expect(router.route("/Foo/Bar")).toEqual({
      name: "r",
      methods: ["GET"],
      params: {},
      query: "",
      anchor: "",
      tags: []
    });
    expect(router.route("/FOO/BAR")).toBeNull();
    expect(router.route("/foo/bar")).toBeNull();
    expect(router.route("/Foo/Bar?QUERY_VaR=FoO")).toEqual({
      name: "r",
      methods: ["GET"],
      params: {},
      query: "?QUERY_VaR=FoO",
      anchor: "",
      tags: []
    });
  });
});

describe('normalizeUrl', () => {
  test('yields expected results', () => {
    const cases = [
      ["", ""],
      ["/foo/bar?boo=blab", "/foo/bar?boo=blab"],
      ["/foo/bar?boo=BLAb", "/foo/bar?boo=BLAb"],
      ["/fOo/bAr?foo=BLAB", "/foo/bar?foo=BLAB"],
      ["/fOo/bAr?foo=BLAB?blah=fUg", "/foo/bar?foo=BLAB?blah=fUg"],
      ["/foo/bar", "/foo/bar"],
      ["/fOo/bAr", "/foo/bar"],
    ];

    for (const [from, to] of cases) {
      expect(normalizeUrl(from)).toEqual(to);
    }
  });
});
