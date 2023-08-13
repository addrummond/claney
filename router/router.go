package router

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type Router struct {
	router router
}

type router struct {
	ConstantPortionRegexp  myRegexp
	ConstantPortionNGroups int
	Families               map[string]family
	Repl                   string
	CaseSensitive          bool
}

type family struct {
	MatchRegexp          myRegexp
	NLevels              int
	NonparamGroupNumbers []int
	Members              []familyMember
}

type familyMember struct {
	Name              string
	ParamGroupNumbers map[string]int
	Tags              []string
	Methods           []string
}

type myRegexp struct { // wrapper to allow custom deserialization
	re *regexp.Regexp
}

func (myRe *myRegexp) UnmarshalJSON(input []byte) error {
	var s string
	err := json.Unmarshal(input, &s)
	if err != nil {
		return err
	}
	myRe.re, err = regexp.Compile(s)
	if err != nil {
		return err
	}
	return nil
}

// MakeRouter constructs a Router from JSON input
func MakeRouter(jsonInput []byte, caseSensitive bool) (Router, error) {
	var r Router

	err := json.Unmarshal(jsonInput, &r.router)
	if err != nil {
		return r, err
	}

	var repl strings.Builder
	repl.WriteRune(' ') // pad output with arbitrary additional initial char so that output will never be equal to input
	for i := 1; i <= r.router.ConstantPortionNGroups; i++ {
		repl.WriteString(fmt.Sprintf("$%v", i))
	}
	r.router.Repl = repl.String()
	r.router.CaseSensitive = caseSensitive

	return r, nil
}

// RouteResult represents the result of attempting to route a URL.
type RouteResult struct {
	Name    string
	Params  map[string]string
	Query   string
	Anchor  string
	Tags    []string
	Methods []string
}

func Route(r *Router, url string) (RouteResult, bool) {
	if !r.router.CaseSensitive {
		url = normalizeUrl(url)
	}

	cp := r.router.ConstantPortionRegexp.re.ReplaceAllString(url, r.router.Repl)
	if cp == url {
		return RouteResult{}, false
	}
	cp = cp[1:] // Remove initial padding char in output

	family, ok := r.router.Families[cp]
	if !ok {
		return RouteResult{}, false
	}

	submatches := family.MatchRegexp.re.FindStringSubmatch(url)
	if submatches == nil {
		return RouteResult{}, false
	}

	groupIndex := findGroupIndex(submatches, family.NonparamGroupNumbers, family.NLevels)

	params := make(map[string]string)
	member := family.Members[groupIndex]

	for paramGroupName, n := range member.ParamGroupNumbers {
		params[paramGroupName] = submatches[n]
	}

	return RouteResult{
		Name:    member.Name,
		Params:  params,
		Query:   submatches[len(submatches)-2],
		Anchor:  submatches[len(submatches)-1],
		Tags:    member.Tags,
		Methods: member.Methods,
	}, true
}

func findGroupIndex(submatches []string, nonParamGroupNumbers []int, nLevels int) int {
	mi := 0 // start of match group range
	nLeaves := 1
	for i := 0; i < nLevels-1; i++ {
		nLeaves *= 2
	}
	gi := 0

	for l := 0; l < nLevels; {
		if len(submatches[nonParamGroupNumbers[mi]]) == 0 {
			// take the right branch
			gi += nLeaves
			mi += nLeaves * 2
		} else {
			// take the left branch
			mi++
		}

		l++
		nLeaves /= 2
	}

	return gi
}

func normalizeUrl(url string) string {
	// The implementation of ToLower already returns the original string in the
	// case where it contains no upper case chars, so there is no point adding
	// that optimization manually.
	q := strings.IndexByte(url, '?')
	if q == -1 {
		return strings.ToLower(url)
	}
	return strings.ToLower(url[0:q]) + url[q:]
}
