import { Router } from './router';

// Keep in sync with value of 'routesJson' var in TestRouter in router/router_test.go
const ROUTE_INFO = {"constantPortionNGroups":34,"constantPortionRegexp":"^(?:(?:\\/+(users)\\/*)(?:|\\/+(?:(\\.)\\/*)|(?:\\/+[^\\/?#]+\\/+(home)\\/*)|(?:\\/+[^\\/?#]+\\/+(profile)\\/*)|(?:\\/+[^\\/?#]+\\/+(orders)\\/*)(?:|(?:\\/+(display)\\/+[^\\/?#]+\\/*)))|(?:\\/+(managers)\\/*)(?:|(?:\\/+[^\\/?#]+\\/+(home)\\/*)|(?:\\/+[^\\/?#]+\\/+(profile)\\/*)|(?:\\/+[^\\/?#]+\\/+(stats)\\/*)|(?:\\/+(orders)\\/+[^\\/?#]+\\/+[^\\/?#]+\\/+(theorder)\\/*)|(?:\\/+(foo)\\/+(goo)\\/+(bar)\\/+[^\\/?#]+\\/*)|(?:\\/+(foo)\\/+(bar)\\/+[^\\/?#]+\\/*)|(?:\\/+(routeending\\\\withbackslash\\\\)\\/*))|(?:\\/+(users)\\/*)(?:|\\/+(?:(foo)\\/*)|(?:\\/+(x)\\/+(y)\\/+(z)\\/+(k)\\/*))|(?:\\/*)(?:|(?:\\/+(foo)\\/+(bar)\\/+[^\\/?#]+\\/*)|(?:\\/+(foobar)\\/+[^\\/?#]+\\/*)|(?:\\/+(f)\\/+(oo)\\/+(bar)\\/+[^\\/?#]+\\/*)|(?:\\/+(fooba)\\/+(r)\\/+[^\\/?#]+\\/*)|(?:\\/+(f)\\/+(oobar)\\/+[^\\/?#]+\\/*)))(?:\\?[^#]*)?(?:#.*)?$","families":{"usersxyzk":{"matchRegexp":"^(?:(\\/+users\\/*\\/+x\\/+y\\/+z\\/+k\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"users/another","paramGroupNumbers":{},"tags":[]}]},"usershome":{"matchRegexp":"^(?:(\\/+users\\/*\\/+([^\\/?#]+)\\/+home\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"users/home","paramGroupNumbers":{"user_id":2},"tags":[]}]},"managersorderstheorder":{"matchRegexp":"^(?:(\\/+managers\\/*\\/+orders\\/+([^\\/?#]+)\\/+([^\\/?#]+)\\/+theorder\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"managers/orders","paramGroupNumbers":{"user_id":2,"o rder_}\\id":3},"tags":["a tag to maybe inherit","baz"]}]},"managersrouteending\\withbackslash\\":{"matchRegexp":"^(?:(\\/+managers\\/*\\/+routeending\\\\withbackslash\\\\\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"managers/backslash","paramGroupNumbers":{},"tags":[]}]},"usersprofile":{"matchRegexp":"^(?:(\\/+users\\/*\\/+([^\\/?#]+)\\/+profile\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"users/profile","paramGroupNumbers":{"user_id":2},"tags":[]}]},"managershome":{"matchRegexp":"^(?:(\\/+managers\\/*\\/+([^\\/?#]+)\\/+home\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"managers/home","paramGroupNumbers":{"manager_id":2},"tags":[]}]},"managersprofile":{"matchRegexp":"^(?:(\\/+managers\\/*\\/+([^\\/?#]+)\\/+profile\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"managers/profile","paramGroupNumbers":{"manager_id":2},"tags":[]}]},"usersfoo":{"matchRegexp":"^(?:(\\/+users\\/*\\/+foo\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"users/foo","paramGroupNumbers":{},"tags":[]}]},"foobar":{"matchRegexp":"^(?:(((\\/*\\/+foo\\/+bar\\/+([^\\/?#]+)\\/*)|(\\/*\\/+foobar\\/+([^\\/?#]+)\\/*))|((\\/*\\/+f\\/+oo\\/+bar\\/+([^\\/?#]+)\\/*)|(\\/*\\/+fooba\\/+r\\/+([^\\/?#]+)\\/*)))|(((\\/*\\/+f\\/+oobar\\/+([^\\/?#]+)\\/*))))(\\?[^#]*)?(#.*)?$","nLevels":3,"nonparamGroupNumbers":[1,2,3,5,7,8,10,12,13,14],"members":[{"name":"dupl/a","paramGroupNumbers":{"param":4},"tags":[]},{"name":"dupl/b","paramGroupNumbers":{"param":6},"tags":[]},{"name":"dupl/c","paramGroupNumbers":{"param":9},"tags":[]},{"name":"dupl/d","paramGroupNumbers":{"param":11},"tags":[]},{"name":"dupl/e","paramGroupNumbers":{"param":15},"tags":[]}]},"managersstats":{"matchRegexp":"^(?:(\\/+managers\\/*\\/+([^\\/?#]+)\\/+stats\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"managers/stats","paramGroupNumbers":{"manager_id":2},"tags":["amp","bar","foo"]}]},"users":{"matchRegexp":"^(?:(\\/+users\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"users","paramGroupNumbers":{},"tags":[]}]},"users.":{"matchRegexp":"^(?:(\\/+users\\/*\\/+\\.\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"users/dot","paramGroupNumbers":{},"tags":[]}]},"usersordersdisplay":{"matchRegexp":"^(?:(\\/+users\\/*\\/+([^\\/?#]+)\\/+orders\\/*\\/+display\\/+([^\\/?#]+)\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"users/orders/order","paramGroupNumbers":{"user_id":2,"order_id":3},"tags":[]}]},"managersfoogoobar":{"matchRegexp":"^(?:(\\/+managers\\/*\\/+foo\\/+goo\\/+bar\\/+([^\\/?#]+)\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"managers/test1","paramGroupNumbers":{"maguffin":2},"tags":[]}]},"managersfoobar":{"matchRegexp":"^(?:(\\/+managers\\/*\\/+foo\\/+bar\\/+([^\\/?#]+)\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"managers/test2","paramGroupNumbers":{"maguffin":2},"tags":["a tag to maybe inherit"]}]}}};
const router = new Router(ROUTE_INFO);

test('yields expected results for various routes', () => {
  expect(router.route("/")).toEqual(null);
  expect(router.route("")).toEqual(null);
  expect(router.route("/users")).toEqual({
    name: "users",
    params: {},
    query: "",
    anchor: "",
    tags: []
  });
  expect(router.route("/users/.")).toEqual({
    name: "users/dot",
    params: {},
    query: "",
    anchor: "",
    tags: []
  });
  expect(router.route("/users/123/home//")).toEqual({
    name: "users/home",
    params: {user_id: "123"},
    query: "",
    anchor: "",
    tags: []
  });
  expect(router.route("/users//123/profile")).toEqual({
    name: "users/profile",
    params: {user_id: "123"},
    query: "",
    anchor: "",
    tags: []
  });
  expect(router.route("/users/123/orders")).toEqual(null);
  expect(router.route("/users/123/orders/display/456")).toEqual({
    name: "users/orders/order",
    params: {user_id: "123", order_id: "456"},
    query: "",
    anchor: "",
    tags: []
  });

  expect(router.route("managers")).toEqual(null);
  expect(router.route("/managers//123/home/")).toEqual({
    name: "managers/home",
    params: {manager_id: "123"},
    query: "",
    anchor: "",
    tags: []
  });
  expect(router.route("/managers/123/stats///")).toEqual({
    name: "managers/stats",
    params: {manager_id: "123"},
    query: "",
    anchor: "",
    tags: ["amp", "bar", "foo"]
  });
  expect(router.route("/managers/orders/123/456/theorder")).toEqual({
    name: "managers/orders",
    params: {user_id: "123", "o rder_}\\id": "456"},
    query: "",
    anchor: "",
    tags: ["a tag to maybe inherit", "baz"]
  });
  expect(router.route("/managers/foo/goo/bar/123")).toEqual({
    name: "managers/test1",
    params: {maguffin: "123"},
    query: "",
    anchor: "",
    tags: []
  });
  expect(router.route("/managers/foo/bar/123")).toEqual({
    name: "managers/test2",
    params: {maguffin: "123"},
    query: "",
    anchor: "",
    tags: ["a tag to maybe inherit"]
  });
  expect(router.route("/managers/routeending\\withbackslash\\")).toEqual({
    name: "managers/backslash",
    params: {},
    query: "",
    anchor: "",
    tags: []
  });

  expect(router.route("/foo//bar/123")).toEqual({
    name: "dupl/a",
    params: {"param": "123"},
    query: "",
    anchor: "",
    tags: []
  });
  expect(router.route("/foobar/123#foo")).toEqual({
    name: "dupl/b",
    params: {"param": "123"},
    query: "",
    anchor: "#foo",
    tags: []
  });
  expect(router.route("/f/oo/bar/123")).toEqual({
    name: "dupl/c",
    params: {"param": "123"},
    query: "",
    anchor: "",
    tags: []
  });
  expect(router.route("/fooba/r/123")).toEqual({
    name: "dupl/d",
    params: {"param": "123"},
    query: "",
    anchor: "",
    tags: []
  });
  expect(router.route("/f/oobar/123")).toEqual({
    name: "dupl/e",
    params: {"param": "123"},
    query: "",
    anchor: "",
    tags: []
  });

  expect(router.route("/managers/123/profile?with=aquery&string=bar")).toEqual({
    name: "managers/profile",
    params: {"manager_id": "123"},
    query: "?with=aquery&string=bar",
    anchor: "",
    tags: []
  });
  expect(router.route("/managers/foo/bar/123?with=aquery&string=foo")).toEqual({
    name: "managers/test2",
    params: {"maguffin": "123"},
    query: "?with=aquery&string=foo",
    anchor: "",
    tags: ["a tag to maybe inherit"]
  });
  expect(router.route("/fooba/r/123#foo?q=a&boo=c")).toEqual({
    name: "dupl/d",
    params: {"param": "123"},
    query: "",
    anchor: "#foo?q=a&boo=c",
    tags: []
  });
  expect(router.route("/f/oobar/123?q=a#foo")).toEqual({
    name: "dupl/e",
    params: {"param": "123"},
    query: "?q=a",
    anchor: "#foo",
    tags: []
  });
});
