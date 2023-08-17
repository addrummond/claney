# Claney – regular routes

Claney is a library for compiling a list of routes down to a set of regular
expressions. A router can be implemented in ~100 lines of code in the language
of your choice.

**Claney is currently beta software. There are reasonably comprehensive tests,
but it has not yet been used in anger.**

## Features

* Guarantees that no two routes overlap.
* Routes can be tagged and output can be limited to routes with a given tag.
* Routing requires only two regular expression operations: a find/replace
  followed by a match.

## Opinions

Claney has some opinions about routes:

* Valid routes must start with `/`.
* A sequence of two or more `/` characters is equivalent to a single `/`.
* Query strings and anchors contain supplemental information and aren't used to
  distinguish between different routes.
* No special treatment of `#!` anchors.

## Limitations

Claney has a minimalist take on what a router is. A router is function that maps
an HTTP method and path string to either 'not found' or

* a name,
* a set of named parameters,
* a set of tags,
* an optional query string and anchor.

In other words, Claney just tells you whether the route exists, which route it
is, and which parameters were supplied. The rest is up to you.

Claney does not directly support matching on hostnames. The 'Hosts' section
covers some options for dealing with multiple hosts. If your routing involves a
complex interaction between hosts and paths, Claney is probably not a good fit.

## Installation and example usage

```sh
go install github.com/addrummond/claney@v0.4.1
$(go env GOPATH)/bin/claney -input my_route_file -output routes.json
```

## Input format

The following is an example of an input file:
```
root          /
users         /users                       [users]
  login       [GET,POST] /login
  settings    /:user_id/settings
managers      /managers                    [managers,admin]
  login       /login
  login       [POST] /login
  settings    /:manager_id/settings
  user        /:manager_id/user/:user_id
  delete      [POST] /:manager_id/delete   [api]
```

This input defines the following routes as valid:

| Route name        | Method       | Path                                | Parameters                   | Tags 
--------------------|--------------|-------------------------------------|------------------------------|------------------------
| root              | GET or POST  | /                                   | `[]`                         | `[]`
| users/login       | GET          | /users/login                        | `[]`                         | `["users"]`
| users/settings    | GET          | /users/:user_id/settings            | `["user_id]"`                | `["users"]`
| managers/login    | GET          | /managers/login                     | `[]`                         | `["admin","managers"]`
| managers/login    | POST         | /managers/login                     | `[]`                         | `["admin","managers"]`
| managers/settings | GET          | /managers/settings                  | `[]`                         | `["admin","managers"]`
| managers/user     | GET          | /managers/:manager_id/user/:user_id | `["manager_id", "user_id"]`  | `["admin","managers"]`
| managers/delete   | POST         | /managers/:manager_id/delete        | `["manager_id", "user_id"]`  | `["admin","api", "managers"]`

By default, `/` is used to join the name of a route to the names of its parent
routes. A different separator may be used if desired (see 'Command line
operation' below).

### General syntax

Tabs or spaces can be used for indentation. It is recommended not to mix tabs
and spaces, but if you do, each tab or space character contributes one unit of
indentation.

Comments are indicated using `#` in the usual way. As the sequence of characters
`:#` is used to specify integer parameters (see below), a `#` is not interpreted
as the beginning of a comment if it is immediately preceded by a `:`.

An input line ending in `\` is joined to the subsequent line (with the `\`
elided) and interpreted as a single logical line.

Special characters can be escaped using `\` (including `\\` for a literal
backslash).

Input files should be UTF-8 encoded. Non-ASCII codepoints may be used as
literals in URL patterns; they are are reproduced as-is inside the regular
expressions.

The output JSON is UTF-8 encoded.

### Route syntax

A route has the following general form:

```
route_name [METHOD1, METHOD2, ...] /path/pattern/:with/:parameters [list,of,tags]
```

Both the list of method names and the list of tags may be omitted. If no methods
are explicitly specified then GET is added by default. Tags are case-sensitive.
Method names are always converted to upper case.

Indentation is significant. If route A is indented under route B then B's path
is joined to A's path and B inherits all of A's tags.

### Route name uniqueness

You may define multiple routes with the same name, but they must be defined
next to each other in the same file.
The following example is OK:

```
posts /users/posts
# ...
# some comments or blank lines may intervene
# ...
posts /users/posts/:id
```

But an error is reported if another route is added in between the two `posts` routes:

```
# BAD
posts          /users/posts
something_else /foo/bar
posts          /users/posts/:id
```

This system retains the flexibility of allowing multiple routes to map to the same
name while making it difficult to accidentally define duplicate route names.

### Named parameters

Named parameters are introduced using the `:` character. Named parameters

* never match empty strings or strings consisting only of `/` characters,
* can appear anywhere within a URL pattern, and
* are matched greedily (except in the case of rest parameters – see below).

#### String parameters

These can be written `:foo`, or `:{foo bar}` to allow whitespace and other
special characters.

#### Integer parameters

Integer parameters are written `:#foo` or `:#{foo bar}`.

#### Rest parameters

Rest parameters are written `:**foo` or `:**{foo bar}`. Unlike normal
parameters, rest parameters can match strings incuding `/` characters.

A rest parameter is typically the final element in a pattern, but you can also
use rest parameters in the middle of a pattern. In this case matching is
non-greedy. For example, the pattern `/foo/:**rest/bar` will match any URL that
starts with `/foo/` and ends with `/bar` – so long as a `rest` can be assigned a
suitable value. For example, `/foo/amp/bar` matches, but `/foo/bar` does not,
and neither does `/foo//bar`.

#### Parameter examples

The following are some examples of URL patterns containing named parameters:

```
/users/:email/summary
/users/:#user_id/comments/:#comment_id
/users/:#user_id/comment-:#comment_id
/users/comment:{uuid}-summary
/users/:**rest
```

### Wildcards

The wildcards `*` and `**` can be used in URL patterns. They are respectively
equivalent to string parameters and rest parameters, except that they are
unnamed and their values are discarded.

### Tags

Tags are enclosed in square brackets after the URL pattern and are separated by
commas. They can contain any characters other than newlines or control
characters. Whitespace and the characters `[],` can be escaped with `\`.

### Trailing slashes

If a route pattern doesn't end with a `/` then a trailing `/` is optional. For
example, the pattern `/users/:id` matches both `/users/123` and `/users/123/`.
If a pattern ends with a `/` then a trailing `/` is obligatory. You can use the
special sequence `!/` to disallow trailing slashes. For example, the pattern
`/users/:id!/` matches `/users/123` but not `/users/123/`.

### Multiple slashes

Claney always treats sequences of multiple slashes as equivalent to a single
slash. For example, `//foo///bar//` is equivalent to `/foo/bar/`.

## Command line operation

Claney reads from stdin and writes to stdout by default. An input file or output
file may be specified using the `-input` and `-output` flags:

```sh
claney < input.routes > output.json
claney -input input.routes -output output.json
```

The `-name-separator` flag may be used to change the separator used to delimit
hierarchical route names. The default is `"/"`.

Claney guarantees that dictionary keys in its JSON output are always serialized
in the same order. Output is therefore guaranteed to be identical for identical
inputs.

### Multiple input files

The `-input` flag can be passed multiple times to generate output on the basis
of multiple input files. The output obtained is essentialy the same as if the
input files were concatenated into one. The only difference is that Claney
considers the least indented route(s) in each file to be at the top level, even
if the least indented route in one file is more indented than the least indented route
in another. In other words, given the following two input files, the router recognizes
`/foo` and `/bar`, not `/foo/bar`:

```
<input file 1>
/foo
```

```
<input file 2>
    /bar
```

### Filtering the output

The output may be filtered to include or exclude routes with certain methods or
tags, e.g.:

```sh
claney -input input.routes -output output.json -include-tags 'manager-*' -exclude-method POST -exclude-method PUT -include-tags special-POST
```

The `-[in/ex]clude-tags` and `-[in/ex]clude-method` flags are interpreted in
order. In the example above, the set of output routes is first restricted to
routes with tags that match the glob `manager-*`. Then all `POST` and `PUT`
routes are excluded from the output set and routes with the `special-post` tag
are added to the output set. If any `POST` routes happen to have the
`special-POST` tag, then these routes will be added back into the output set,
overriding their previous exclusion.

In the case of routes with multiple methods, each method is treated
independently for filtering. For example, in the case of a route such as `foo
[GET,POST] /foo`, the flag `-exclude-method POST` generates a router that
recognizes `GET /foo` but not `POST /foo`.

If the first flag in the sequence is an include, then the initial output route
set contains all and only the routes included by that flag. If the first flag is
an exclude, then the initial set contains all routes except those excluded by
the flag.

### Adding a prefix to the output

You can add a prefix to the output using the `-output-prefix` flag. This is
handy if you want to output the routes as a JS file. For example:

```
claney -input input.routes -output-prefix 'export const ROUTES=' -output output.js
```

## Hosts

Claney does not directly support matching on hostnames. If your routing involves
a complex interaction between hosts and paths, Claney is probably not a good
fit.

For simple cases there are two workable options:

* Define a separate router for each host. This makes sense for cases such as
  `foo.com` and `api.foo.com`.
* Tag each route with the host(s) where it is valid.

An example of the second option is the following:

```
routeA /foo [host:host1.foo.com, host:host2.foo.com]
routeB /bar [host:host1.foo.com]
```

You can add logic in your router to 404 if the host doesn't match one of the
`host:*` tags. Alternatively, you can use filtering to generate a separate
router for each host:

```sh
claney -input routes -include-tags 'host:host1.foo.com' -output just_host1.json
claney -input routes -include-tags 'host:host2.foo.com' -output just_host2.json
claney -input routes -include-tags 'host:*' -output all_hosts.json
```

## Case sensitivity

By default Claney operates in case-insensitive mode. Routes must be defined
using only lower case characters, and the example router implementations
normalize URLs to lower case before matching (excluding the query string and
anchor).

If you want your router to be case-sensitive, pass the `-allow-upper-case` flag
to `claney` and ensure that to your router implementation does not normalize URLs
before matching. The router constructors in the example Go and JavaScript router
implementations take a boolean `caseSensitive` parameter.

Benchmarking suggests that the cost of normalization is trivial (in JavaScript
and Go at least) compared to the rest of the computation required to match a
route.

## Implementation

Routing is a two-step process. The first step is a find/replace  using a single
'God' regular expression that matches valid routes. The result of the
find/replace is a string containing all of the constant portions of the input
route. For example, if the route is something like `/manager/10/settings`, then
the constant portion would be `/manager//settings`. In most cases, the constant
portion uniquely identifies a route; if so, a dictionary lookup is performed to
retrieve a regex for matching the route string and extracting its parameter
values. If multiple routes share the same constant portion then the matching
regex is disjunctive and matches all applicable routes. The route can then be
identified by the indices of the groups that have non-empty captures.

In the regexp used for the second step, capture groups are nested according to
the scheme of a binary tree. For example, suppose that there are six routes
(R1...R6) in a group. The portion of the regexp corresponding to each route is
wrapped in a capture group. The smallest complete binary tree that can hold 6
values in its leaf nodes has a depth of 3. The capture groups are therefore
nested as follows:

```
(                            )(                           )
 (            )(            )  (    R5     ) (    R6     )
  ( R1 )( R2 )  ( R3 )( R4 )
```

The matching route (if any) can be located via binary search. For example, if
the leftmost capture group on the first line is empty then we know that either
R5 or R6 is the matching route.

There are some edge cases where an invalid route will match the initial 'God'
regular expression but then fail to match the second regular expression. Routers
should interpret this scenario as a 404.

## Performance

Claney generates a single disjunctive regex representing the entire set of valid
routes. Regex engines are generally not well optimized for massively disjunctive
regular expressions. Even trivial cases such as `foo|bar|foobar|baz` will
often trigger unnecessary backtracking and performance linear in the number of
alternatives.

Claney does a couple of things to make its disjunctive regular expressions
execute as quickly as possible. First, if routes are specified hierachically in
the input file, the common prefixes are factored out in the regular expression.
This limits the amount of backtracking required.

Second, in the case of alternatives at the same level of the hierarchy, Claney
will factor out common prefixes automatically. For example, rather than
generating a regular expression such as `apple|artichoke|pear|plum`, it
generates `(a(pple|rtichoke))|(p(ear|lum))`.

The Javascript benchmark in `js/router.bench.js` gives a rough idea of the
performance that can be expected. The input file contains *n* routes of the form
`/${m}foo` for *m*=1..*n* (a fairly pessimal case, given the lack of hierarchy).
On an M1 Macbook Air, the following times per routing operation are observed:

```
10 routes:    0.0003  milliseconds (per routing operation)
100 routes:   0.0004  milliseconds
1000 routes:  0.0019  milliseconds
10000 routes: 0.0164  milliseconds
```

## Decomposing routers

Claney does not provide any special facilty for 'including' one router inside
another, but it is easy to use rest parameters to decompose one router into
multiple subrouters. For example:

```
<file main_routes>
  managers /managers/:**{url} [managers]
  # add the line below if you want `/managers` to be a valid route,
  # as rest parameters do not match empty strings.
  managers /managers          [managers]

  clients /clients/:**{url} [clients]

<file manager_routes>
  foo /foo
  bar /bar

<file client_routes>
  amp /amp
  baz /baz

<router code>
  const mainRouter = new Router(MAIN_ROUTES_JSON);
  const managerRouter = new Router(MANAGER_ROUTES_JSON);
  const clientRouter = new Router(CLIENT_ROUTES_JSON);

  function route(url) {
    const r = mainRouter.route(url);
    if (r === null)
      return null;
    const subrouteUrl = r.params.url || '/';

    // you might want to add additional metadata to the return value
    // to indicate which router matched the route.
    if (r.tags.indexOf("managers") !== -1)
      return managerRouter.route(subrouteUrl);
    if (r.tags.indexOf("clients") !== -1)
      return clientRouter.route(subrouteUrl);
    return null;
  }
```

## Example implementations

Javascript and Go router implementations are provided in `js/router.js` and
`router/router.go`.

There is a simple example of integrating the router into a single-page React app
in `js/react_example`. In this directory, run `build.sh` and then
`startserver.sh` to start a server on localhost:80001. (CORS issues make it
necessary to use a server.) The app uses the library in
`js/reactmicrorouter.js`.

## Name

Claney is named after [Stephen Cole Kleene](https://en.wikipedia.org/wiki/Stephen_Cole_Kleene) (whose last name is pronounced [ˈkleɪni]).
