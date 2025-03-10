package compiler

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

type RouteInfo struct {
	Name     string
	Line     int
	Filename string
	Tags     map[string]struct{}
	Depth    int
	Terminal bool
	Methods  map[string]struct{}
}

type CompiledRoute struct {
	Info     RouteInfo
	Compiled RouteRegexp
}

type RouteRegexp struct {
	Elems                 []routeElement
	MatchRegexp           string
	ConstantPortionRegexp func(int) string
	ConstantPortion       string
	ConstishPrefix        string // constant bar allowance of repeated '/', etc.
	ConstishSuffix        string
	NGroups               int
	ParamGroupNumbers     map[string]int
}

type routeFamily struct {
	constantPortion      string
	members              []routeGroupMember
	nonparamGroupNumbers []int
	nLevels              int
	matchRegexp          string
}

type routeGroupMember struct {
	name              string
	route             *RouteWithParents
	paramGroupNumbers map[string]int
}

type routeRegexps struct {
	constantPortionRegexp  string
	constantPortionNGroups int
	families               []routeFamily
}

type RouteWithParents struct {
	Route   *CompiledRoute
	Parents []*CompiledRoute
}

func routeToRegexps(elems []routeElement) RouteRegexp {
	var re strings.Builder // the match regex
	var cp strings.Builder // the replacement regex taking us to the constant portion
	var constantPortion strings.Builder
	var constantPortionI int
	var constishPrefix strings.Builder
	var constishSuffix strings.Builder
	inConstishPrefix := true

	// If there's an initial and non-final '/' we don't include this in the regex, as it makes
	// it a pain to join the regexes of hierarchically nested routes.
	if len(elems) > 0 && elems[0].kind == slash {
		elems = elems[1:]
	}

	paramGroupNumbers := make(map[string]int)
	groupI := 1
	constantPortionNGroups := 0
	for i, elem := range elems {
		switch elem.kind {
		case slash:
			if i+1 == len(elems) {
				continue
			}
			re.WriteString("\\/+")
			cp.WriteString("(\\/)\\/*")
			constantPortionNGroups++
			constantPortion.WriteRune('/')
			if inConstishPrefix {
				constishPrefix.WriteRune('/')
			}
			constishSuffix.WriteRune('/')
		case constant:
			regexEscape(elem.value, &re)

			constantPortion.WriteString(elem.value)
			constantPortionI++

			// Don't include the first constant in the constant portion regexp. This
			// is filled in later after factoring
			if i != 0 {
				cp.WriteByte('(')
				regexEscape(elem.value, &cp)
				cp.WriteByte(')')
				constantPortionNGroups++
			}

			if inConstishPrefix {
				constishPrefix.WriteString(elem.value)
			}
			constishSuffix.WriteString(elem.value)
		case parameter:
			re.WriteString("([^\\/?#]+)")
			cp.WriteString("[^\\/?#]+")
			paramGroupNumbers[elem.value] = groupI
			groupI++
			inConstishPrefix = false
			constishSuffix.Reset()
		case integerParameter:
			re.WriteString("(-?[0-9]+)")
			cp.WriteString("-?[0-9]+")
			paramGroupNumbers[elem.value] = groupI
			groupI++
			inConstishPrefix = false
			constishSuffix.Reset()
		case restParameter:
			// Rest parameters are a little more complex than you might think because
			// we don't want them to match strings consisting entirely of slashes.
			// (This would end up violating our rule that multiple sequences of
			// slashes in a URL are always equivalent to just a single slash.)

			// Make it non-greedy if it's not at the end
			if i+1 == len(elems) {
				re.WriteString("(\\/*[^\\/?#][^?#]*)")
				cp.WriteString("\\/*[^\\/?#][^?#]*")
			} else {
				re.WriteString("(\\/*[^\\/?#][^?#]*?)")
				cp.WriteString("\\/*[^\\/?#][^?#]*?")
			}
			paramGroupNumbers[elem.value] = groupI
			groupI++
			inConstishPrefix = false
			constishSuffix.Reset()
		case singleGlob:
			common := "[^\\/?#]+"
			re.WriteString(common)
			cp.WriteString(common)
			inConstishPrefix = false
			constishSuffix.Reset()
		case doubleGlob:
			// Make it non-greedy if it's not at the end
			if i+1 == len(elems) {
				common := "([^?#]+)"
				re.WriteString(common)
				cp.WriteString(common)
			} else {
				// See comment above for rest params for why this regexp is relatively complex.
				common := "(\\/*[^\\/?#][^?#]+?)"
				cp.WriteString(common)
				re.WriteString(common)
			}
			groupI++
			inConstishPrefix = false
			constishSuffix.Reset()
		case noTrailingSlash:
			if i+1 != len(elems) {
				panic("What's a 'no trailing slash' element doing here?!")
			}
		}
	}

	cpRegexString := cp.String()
	var firstConstant string
	if len(elems) > 0 && elems[0].kind == constant {
		firstConstant = elems[0].value
	}

	return RouteRegexp{
		Elems:           elems,
		ConstantPortion: constantPortion.String(),
		MatchRegexp:     re.String(),
		ConstantPortionRegexp: func(offset int) string {
			if offset > len(firstConstant) {
				panic("Bad offset given to 'constantPortionRegexp' function member of 'RouteInfo'")
			}
			fc := firstConstant[offset:]
			if fc != "" {
				var out strings.Builder
				out.WriteByte('(')
				regexEscape(fc, &out)
				out.WriteByte(')')
				out.WriteString(cpRegexString)
				return out.String()
			}
			return cpRegexString
		},
		ConstishPrefix:    constishPrefix.String(),
		ConstishSuffix:    constishSuffix.String(),
		NGroups:           groupI - 1,
		ParamGroupNumbers: paramGroupNumbers,
	}
}

func regexEscape(str string, sb *strings.Builder) {
	for i := range str {
		c := str[i]
		if strings.ContainsRune(regexpSpecialChars, rune(c)) {
			sb.WriteByte('\\')
		}
		sb.WriteByte(c)
	}
}

type tne struct {
	file      string
	line      int
	fileIndex int
	entryInex int
}

func ProcessRouteFiles(files [][]RouteFileEntry, filenames []string, nameSeparator string) ([]CompiledRoute, []RouteError) {
	if len(files) != len(filenames) {
		panic("Error in 'ProcessRouteFile': files and filenames args must be of the same length")
	}

	var errors []RouteError

	routes := make([]CompiledRoute, 0)

	terminalLines := make(map[string][]tne)
	linesWithEntries := make(map[int]struct{})

	type level struct {
		name      string
		baseRoute []routeElement
		indent    int
	}

	levels := make([]level, 0)

	for fi, file := range files {
		for ei, entry := range file {
			linesWithEntries[entry.line] = struct{}{}

			var li int
			for li = len(levels) - 1; li >= 0; li-- {
				if levels[li].indent < entry.indent {
					break
				}
			}
			levels = levels[:li+1]

			var nameB strings.Builder
			for li, lev := range levels {
				if li != 0 {
					nameB.WriteString(nameSeparator)
				}
				nameB.WriteString(lev.name)
			}
			if len(levels) > 0 {
				nameB.WriteString(nameSeparator)
			}
			nameB.WriteString(entry.name)

			name := nameB.String()
			if entry.terminal {
				terminalLines[name] = append(terminalLines[name], tne{filenames[fi], entry.line, fi, ei})
			}

			cri := routeToRegexps(entry.pattern)
			ri := CompiledRoute{
				Info: RouteInfo{Name: name,
					Depth:    len(levels),
					Line:     entry.line,
					Filename: filenames[fi],
					Tags:     entry.tags,
					Methods:  entry.methods,
					Terminal: entry.terminal,
				},
				Compiled: cri,
			}

			levels = append(levels, level{entry.name, entry.pattern, entry.indent})

			routes = append(routes, ri)
		}
	}

	errors = append(errors, checkNonadjacentNamesakes(terminalLines, linesWithEntries)...)

	return routes, errors
}

func CheckForGroupErrors(routes []CompiledRoute) (errors []RouteError) {
	var terminals []RouteWithParents
	withParentRoutes(routes, func(r *CompiledRoute, parents []*CompiledRoute) {
		if r.Info.Terminal {
			terminals = append(terminals, RouteWithParents{r, parents})
		}
	})
	groupedRoutes := GroupRoutes(terminals)
	errors = append(errors, checkForOverlaps(groupedRoutes)...)

	for _, rwps := range groupedRoutes {
		if len(rwps) > BiggestOverlapGroupAllowedBeforeWarning {
			errors = append(errors, RouteError{
				Kind:  WarningBigGroup,
				Line:  rwps[0].Route.Info.Line,
				Col:   1,
				Group: rwps,
			})
		}
	}

	return
}

func FindNonterminalRoutesWithoutChildren(routes []CompiledRoute) (withoutChildren []int) {
	for i, r := range routes {
		if !r.Info.Terminal {
			if i+1 >= len(routes) || routes[i+1].Info.Depth <= r.Info.Depth {
				withoutChildren = append(withoutChildren, i)
			}
		}
	}
	return
}

func checkNonadjacentNamesakes(terminalLines map[string][]tne, linesWithEntries map[int]struct{}) []RouteError {
	var errors []RouteError

	// Check for any terminal routes with the same name that aren't adjacent in the file.
	for name, lines := range terminalLines {
		if len(lines) <= 1 {
			continue
		}

		sort.Slice(lines, func(i, j int) bool { return lines[i].line < lines[j].line })
		for i := 0; i < len(lines)-1; i++ {
			if lines[i].file != lines[i+1].file {
				errors = append(errors, RouteError{
					Kind:          DuplicateRouteName,
					Line:          lines[i].line,
					Col:           -1,
					DuplicateName: name,
					OtherLine:     lines[i+1].line,
					Filenames:     []string{lines[i].file, lines[i+1].file},
				})
				break
			}
			for l := lines[i].line + 1; l < lines[i+1].line; l++ {
				if _, ok := linesWithEntries[l]; ok {
					errors = append(errors, RouteError{
						Kind:          DuplicateRouteName,
						Line:          lines[i].line,
						Col:           -1,
						DuplicateName: name,
						OtherLine:     lines[i+1].line,
						Filenames:     []string{lines[i].file},
					})
					break
				}
			}
		}
	}

	return errors
}

type overlapBetween struct {
	route1, route2 *CompiledRoute
}

const BiggestOverlapGroupAllowedBeforeWarning = 5
const MaxOverlapGroupErrors = 10

func GroupRoutes(rwps []RouteWithParents) [][]RouteWithParents {
	byPrefix := groupByConstishPrefix(rwps)
	byPrefixAndSuffix := make([][]RouteWithParents, 0)
	for _, routes := range byPrefix {
		byPrefixAndSuffix = append(byPrefixAndSuffix, groupByConstishSuffix(routes)...)
	}
	return byPrefixAndSuffix
}

func checkForOverlaps(grouped [][]RouteWithParents) []RouteError {
	var errors []RouteError
	for _, routes := range grouped {
		os := checkForOverlapsWithinGroup(routes)

		for _, o := range os {
			errors = append(errors, RouteError{
				Kind:      OverlappingRoutes,
				Line:      o.route1.Info.Line,
				Col:       -1,
				OtherLine: o.route2.Info.Line,
				Filenames: []string{o.route1.Info.Filename, o.route2.Info.Filename},
			})
		}

		if len(errors) > MaxOverlapGroupErrors {
			break
		}
	}

	return errors
}

func checkForOverlapsWithinGroup(rwps []RouteWithParents) []overlapBetween {
	regexps := make([]*node, 0)
	regexpToInfo := make(map[*node]*CompiledRoute)

	for _, rwp := range rwps {
		var resb strings.Builder
		resb.WriteString("\\/+")
		for i, p := range rwp.Parents {
			if i != 0 {
				resb.WriteString("\\/+")
			}
			resb.WriteString(p.Compiled.MatchRegexp)
		}
		if len(rwp.Parents) > 0 {
			resb.WriteString("\\/+")
		}
		resb.WriteString(rwp.Route.Compiled.MatchRegexp)
		resb.WriteString(routeTerm(rwp.Route))

		regexp, err := regexpToNfa(resb.String())
		if err != nil {
			panic(fmt.Sprintf("Internal error compiling regexp in 'checkForOverlap': %v", err))
		}

		regexps = append(regexps, regexp)
		regexpToInfo[regexp] = rwp.Route
	}

	overlapIndices := findOverlaps(regexps)
	overlaps := make([]overlapBetween, 0, len(overlapIndices))
	for i := range overlapIndices {
		oi1, oi2 := overlapIndices[i].i1, overlapIndices[i].i2
		ri1, ri2 := regexpToInfo[regexps[oi1]], regexpToInfo[regexps[oi2]]

		// Ignore this overlap if the methods don't overlap.
		methodInCommon := false
		for m1 := range ri1.Info.Methods {
			if _, ok := ri2.Info.Methods[m1]; ok {
				methodInCommon = true
				break
			}
		}
		if methodInCommon {
			overlaps = append(overlaps, overlapBetween{regexpToInfo[regexps[oi1]], regexpToInfo[regexps[oi2]]})
		}
	}

	return overlaps
}

func withParentRoutes(routes []CompiledRoute, iter func(*CompiledRoute, []*CompiledRoute)) {
	lastLevel := 0
	parentRoutes := make([]*CompiledRoute, 0)

	for i := range routes {
		r := &routes[i]

		if r.Info.Depth > lastLevel && i > 0 {
			parentRoutes = append(parentRoutes, &routes[i-1])
		} else if r.Info.Depth < lastLevel {
			parentRoutes = parentRoutes[0 : len(parentRoutes)-(lastLevel-r.Info.Depth)]
		}

		cp := make([]*CompiledRoute, len(parentRoutes))
		copy(cp, parentRoutes)
		iter(r, cp)

		lastLevel = r.Info.Depth
	}
}

func withParentRoutesFromTree(tree *cpNode, iter func(*CompiledRoute, []*CompiledRoute)) {
	var rec func(n *cpNode, parents []*CompiledRoute)
	rec = func(n *cpNode, parents []*CompiledRoute) {
		if n == nil { // nil nodes can be inserted during some optimization passes
			return
		}

		if n.routeInfo != nil {
			iter(n.routeInfo, parents)
		}

		for _, c := range n.children {
			var ps []*CompiledRoute
			ps = append(ps, parents...)
			if n.routeInfo != nil {
				ps = append(ps, n.routeInfo)
			}
			rec(c, ps)
		}
	}

	rec(tree, nil)
}

type familyWithConstantPortion struct {
	constantPortion string
	routes          []RouteWithParents
}

func familiesByConstantPortion(n *cpNode) []familyWithConstantPortion {
	families := make(map[string][]RouteWithParents, 0)
	constantPortionUpTo := make(map[*CompiledRoute]string)

	withParentRoutesFromTree(n, func(r *CompiledRoute, parents []*CompiledRoute) {
		var cpb strings.Builder
		for i, p := range parents {
			if i != 0 && !isJustSlash(parents[i-1]) {
				cpb.WriteString("/")
			}
			cpb.WriteString(p.Compiled.ConstantPortion)
		}
		if len(parents) != 0 && !isJustSlash(parents[len(parents)-1]) {
			cpb.WriteString("/")
		}
		cpb.WriteString(r.Compiled.ConstantPortion)
		cp := cpb.String()
		constantPortionUpTo[r] = cp

		families[cp] = append(families[cp], RouteWithParents{r, parents})
	})

	// For determinism
	cps := make([]string, 0)
	for cp := range families {
		cps = append(cps, cp)
	}
	sort.Strings(cps)

	fwcps := make([]familyWithConstantPortion, 0)
	for _, cp := range cps {
		fwcps = append(fwcps, familyWithConstantPortion{cp, families[cp]})
	}

	return fwcps
}

func filterTreeByTags(n *cpNode, filter *TagExpr) {
	// Mark all routes to be excluded and remove children of any wholly excluded
	// subtrees.
	var rec func(n *cpNode)
	rec = func(n *cpNode) {
		ci := 0
		for _, c := range n.children {
			if EvalTagExpr(filter, c.routeInfo.Info.Tags, c.routeInfo.Info.Methods) {
				n.children[ci] = c
				ci++
			} else {
				c.excluded = true
				c.routeInfo.Info.Terminal = false // ensure route does not appear in output
				if len(c.children) != 0 {
					n.children[ci] = c
					ci++
				}
			}
			rec(c)
		}
		n.children = n.children[:ci]
	}

	rec(n)

	// Remove children of wholly excluded subtrees
	var excl func(n *cpNode) bool
	excl = func(n *cpNode) bool {
		ci := 0
		allExcluded := true
		for _, c := range n.children {
			if !excl(c) {
				allExcluded = false
				n.children[ci] = c
				ci++
			}
		}
		n.children = n.children[:ci]
		return n.excluded && allExcluded
	}

	excl(n)
}

func GetRouteRegexps(routes []CompiledRoute, filter *TagExpr) routeRegexps {
	tree := getConstantPortionTree(routes)

	filterTreeByTags(tree, filter)
	optimizeConstantPortionTree(tree)

	originalConstantPortionRegexp := getConstantPortionRegexp(tree)
	parsedConstantPortionRegexp := parseRegexp(originalConstantPortionRegexp)
	scratchBuffer := make([]byte, 64)
	sgds := findSingleGroupDisjuncts(parsedConstantPortionRegexp, scratchBuffer)
	refactorSingleGroupDisjuncts(sgds)
	constantPortionRegexp := renodeToString(parsedConstantPortionRegexp)

	byCp := familiesByConstantPortion(tree)
	families := make([]routeFamily, 0)

	totalNGroups := getNCaptureGroups(constantPortionRegexp)

	for _, fwcp := range byCp {
		cp := fwcp.constantPortion
		cpRoutes := fwcp.routes

		ts := getTerminalRoutes(cpRoutes)
		if len(ts) == 0 {
			continue
		}

		result := disjoinRegexp(ts)

		members := make([]routeGroupMember, 0)
		for i := range result.paramGroups {
			members = append(members, routeGroupMember{
				name:              result.names[i],
				paramGroupNumbers: result.paramGroups[i],
				route:             ts[i],
			})
		}

		families = append(families, routeFamily{
			constantPortion:      cp,
			members:              members,
			matchRegexp:          wrapMatchRegexp(result.regex),
			nonparamGroupNumbers: result.nonparamGroups,
			nLevels:              result.nLevels,
		})
	}

	return routeRegexps{
		constantPortionRegexp:  wrapConstantPortionRegexp(constantPortionRegexp),
		constantPortionNGroups: totalNGroups,
		families:               families,
	}
}

func getTerminalRoutes(rs []RouteWithParents) []*RouteWithParents {
	terms := make([]*RouteWithParents, 0)
	for i, r := range rs {
		if r.Route.Info.Terminal {
			terms = append(terms, &rs[i])
		}
	}
	return terms
}

type InclusionStatus int

const (
	Include InclusionStatus = iota
	Exclude InclusionStatus = iota
	Union   InclusionStatus = iota
)

func RouteRegexpsToJSON(rrs *routeRegexps, filter *TagExpr) ([]byte, int) {
	// This function outputs the JSON directly without building an intermediate
	// data structure. It's slightly more fiddly, but saves on unnecessary
	// allocation.

	out := make([]byte, 0, 1024)

	out = append(out, `{"constantPortionNGroups":`...)
	out = appendJsonPosInt(out, rrs.constantPortionNGroups)
	out = append(out, `,"constantPortionRegexp":`...)
	out = appendJsonString(out, rrs.constantPortionRegexp)
	out = append(out, `,"families":{`...)

	nFamiliesOut := 0
	nRoutesOut := 0
	for _, g := range rrs.families {
		if nFamiliesOut != 0 {
			out = append(out, ',')
		}
		nFamiliesOut++

		out = appendJsonString(out, g.constantPortion)
		out = append(out, `:{"matchRegexp":`...)
		out = appendJsonString(out, g.matchRegexp)
		out = append(out, `,"nLevels":`...)
		out = appendJsonPosInt(out, g.nLevels)
		out = append(out, `,"nonparamGroupNumbers":[`...)
		for j, npg := range g.nonparamGroupNumbers {
			if j != 0 {
				out = append(out, ',')
			}
			out = appendJsonPosInt(out, npg)
		}
		out = append(out, `],"members":[`...)
		nMembersOut := 0
		for _, m := range g.members {
			matchingMs := matchingMethods(filter, m.route.Route.Info.Methods, m.route.Route.Info.Tags)
			if len(matchingMs) == 0 {
				continue
			}
			if nMembersOut != 0 {
				out = append(out, ',')
			}
			nMembersOut++
			nRoutesOut++
			out = append(out, `{"name":`...)
			out = appendJsonString(out, m.name)
			out = append(out, `,"paramGroupNumbers":{`...)
			k := 0
			for key, pgn := range m.paramGroupNumbers {
				if k != 0 {
					out = append(out, ',')
				}
				out = appendJsonString(out, key)
				out = append(out, ':')
				out = appendJsonPosInt(out, pgn)
				k++
			}
			out = append(out, `},"tags":[`...)
			for k, tag := range computeTags(&m) {
				if k != 0 {
					out = append(out, ',')
				}
				out = appendJsonString(out, tag)
			}
			out = append(out, `],"methods":[`...)
			for k, m := range stringSetToList(matchingMs) {
				if k != 0 {
					out = append(out, ',')
				}
				out = appendJsonString(out, m)
			}
			out = append(out, "]}"...)
		}
		out = append(out, `]}`...)
	}

	out = append(out, `}}`...)

	return out, nRoutesOut
}

func matchingMethods(filter *TagExpr, methods map[string]struct{}, tags map[string]struct{}) map[string]struct{} {
	r := make(map[string]struct{})
	for m := range methods {
		if EvalTagExpr(filter, tags, map[string]struct{}{m: {}}) {
			r[m] = struct{}{}
		}
	}
	return r
}

func computeTags(m *routeGroupMember) []string {
	tags := make(map[string]struct{}, len(m.route.Route.Info.Tags)*2)
	for k := range m.route.Route.Info.Tags {
		tags[k] = struct{}{}
	}
	for _, p := range m.route.Parents {
		for k := range p.Info.Tags {
			tags[k] = struct{}{}
		}
	}
	return stringSetToList(tags)
}

func stringSetToList(tags map[string]struct{}) []string {
	lst := make([]string, len(tags))
	i := 0
	for tag := range tags {
		lst[i] = tag
		i++
	}
	sort.Strings(lst)
	return lst
}

type disjoinRegexResult struct {
	regex          string
	paramGroups    []map[string]int
	names          []string
	nonparamGroups []int
	nLevels        int
}

func disjoinRegexp(routes []*RouteWithParents) disjoinRegexResult {
	nLevels := 1
	nLeaves := 2
	for nLeaves < len(routes) {
		nLevels++
		nLeaves *= 2
	}

	var sb strings.Builder

	sb.WriteString("(?:")

	paramGroups := make([]map[string]int, len(routes))
	names := make([]string, len(routes))
	nonparamGroups := make([]int, 0)
	currentGroupNumber := 1
	balance := 0
	for i, r := range routes {
		if i != 0 {
			sb.WriteRune('|')
		}

		m := 2
		for j := 0; j < nLevels-1; j++ {
			if i%m == 0 {
				sb.WriteRune('(')
				balance++
				nonparamGroups = append(nonparamGroups, currentGroupNumber)
				currentGroupNumber++
			}
			m *= 2
		}

		sb.WriteString("(\\/+")
		nonparamGroups = append(nonparamGroups, currentGroupNumber)
		currentGroupNumber++

		for j, p := range r.Parents {
			if j != 0 && !isJustSlash(r.Parents[j-1]) {
				sb.WriteString("\\/+")
			}

			sb.WriteString(p.Compiled.MatchRegexp)
		}
		if len(r.Parents) > 0 && !isJustSlash(r.Parents[len(r.Parents)-1]) {
			sb.WriteString("\\/+")
		}
		sb.WriteString(r.Route.Compiled.MatchRegexp)
		sb.WriteString(routeTerm(r.Route))

		names[i] = r.Route.Info.Name

		paramGroups[i] = make(map[string]int)
		for _, p := range r.Parents {
			for k, v := range p.Compiled.ParamGroupNumbers {
				paramGroups[i][k] = currentGroupNumber + v - 1 // groups numbered from 1
			}
			currentGroupNumber += p.Compiled.NGroups
		}

		for k, v := range r.Route.Compiled.ParamGroupNumbers {
			paramGroups[i][k] = currentGroupNumber + v - 1 // groups numbered from 1
		}
		currentGroupNumber += r.Route.Compiled.NGroups

		sb.WriteRune(')')

		m = 2
		for j := 0; j < nLevels-1; j++ {
			if i%m == m-1 {
				sb.WriteRune(')')
				balance--
			}
			m *= 2
		}
	}

	for i := 0; i < balance; i++ {
		sb.WriteRune(')')
	}

	sb.WriteString(")")

	return disjoinRegexResult{
		regex:          sb.String(),
		paramGroups:    paramGroups,
		names:          names,
		nonparamGroups: nonparamGroups,
		nLevels:        nLevels,
	}
}

type cpNode struct {
	routeInfo  *CompiledRoute
	leftOffset int
	factorChar rune // 0 if regular node
	excluded   bool // used temporarily during filtering
	children   []*cpNode
}

func getConstantPortionTree(routes []CompiledRoute) *cpNode {
	var root cpNode
	currentParent := &root
	parents := []*cpNode{currentParent}
	lastLevel := 0

	nodes := make([]cpNode, 0, len(routes)*4)
	makeNode := func(r *CompiledRoute) *cpNode {
		if cap(nodes) <= len(nodes) {
			nodes = make([]cpNode, 0, cap(nodes)*2)
		}
		nodes = append(nodes, cpNode{routeInfo: r})
		return &nodes[len(nodes)-1]
	}

	for i := range routes {
		r := &routes[i]

		if r.Info.Depth == lastLevel {
			currentParent.children = append(currentParent.children, makeNode(r))
		} else if r.Info.Depth > lastLevel {
			if len(currentParent.children) == 0 {
				panic("Internal error in 'getConstantPortionTree' [1]")
			}
			last := currentParent.children[len(currentParent.children)-1]
			parents = append(parents, last)
			last.children = append(last.children, makeNode(r))
			currentParent = last
		} else { // if r.depth < lastLevel
			var i int
			for i = len(parents) - 1; i >= 1 && r.Info.Depth <= parents[i].routeInfo.Info.Depth; i-- {
			}
			if len(parents) == 0 {
				panic("Internal error in 'getConstantPortionTree' [2]")
			}
			currentParent = parents[i]
			parents = parents[:i+1]
			currentParent.children = append(currentParent.children, makeNode(r))
		}

		lastLevel = r.Info.Depth
	}

	return &root
}

func optimizeConstantPortionTree(tree *cpNode) {
	if tree == nil {
		return
	}
	if len(tree.children) == 0 {
		return
	}

	sort.Slice(tree.children, func(i, j int) bool {
		elem1 := tree.children[i]
		elem2 := tree.children[j]
		var c1, c2 string
		if len(elem1.routeInfo.Compiled.Elems) != 0 && elem1.routeInfo.Compiled.Elems[0].kind == constant {
			c1 = elem1.routeInfo.Compiled.Elems[0].value[elem1.leftOffset:]
		}
		if len(elem2.routeInfo.Compiled.Elems) != 0 && elem2.routeInfo.Compiled.Elems[0].kind == constant {
			c2 = elem2.routeInfo.Compiled.Elems[0].value[elem2.leftOffset:]
		}
		if c2 == "" {
			return false
		}
		if c1 == "" {
			return c2 != ""
		}
		// We could just compare the first rune of each string, but then the order
		// wouldn't be deterministic.
		return c1 < c2
	})

	nByFirstChar := make(map[rune]int)
	firstCharStartIndices := make(map[rune]int)
	firstCharEndIndices := make(map[rune]int)
	var fc, lastChar rune
	for i, c := range tree.children {
		fc = getFirstChar(c.routeInfo, c.leftOffset)

		if fc == 0 {
			nByFirstChar[0]++
			continue
		}

		nByFirstChar[fc]++
		if _, ok := firstCharStartIndices[fc]; !ok {
			firstCharStartIndices[fc] = i
		}

		if fc != lastChar {
			firstCharEndIndices[lastChar] = i
		}
		lastChar = fc
	}
	if len(tree.children) > 0 {
		firstCharEndIndices[fc] = len(tree.children)
	}

	if len(nByFirstChar) >= 3 {
		for fc, n := range nByFirstChar {
			if fc == 0 || n < 2 {
				continue
			}

			si, ei := firstCharStartIndices[fc], firstCharEndIndices[fc]

			children := make([]*cpNode, ei-si)
			copy(children, tree.children[si:ei])
			tree.children[si] = &cpNode{factorChar: fc, children: children}
			for i := si + 1; i < ei; i++ {
				tree.children[i] = nil
			}
			for _, c := range children {
				c.leftOffset += utf8.RuneLen(fc)
			}

		}
	}

	for _, c := range tree.children {
		optimizeConstantPortionTree(c)
	}
}

func getConstantPortionRegexp(tree *cpNode) string {
	var sb strings.Builder
	sb.WriteString("(?:\\/+(?:")

	var rec func(*cpNode, bool)
	rec = func(n *cpNode, addTerm bool) {
		if n == nil {
			return
		}

		commonTerm := "X"
		for _, c := range n.children {
			if !addTerm || n.routeInfo != nil && !n.routeInfo.Info.Terminal {
				break
			}
			if c == nil {
				continue
			}

			if c.routeInfo == nil || !c.routeInfo.Info.Terminal || isJustSlash(c.routeInfo) {
				commonTerm = "X"
				break
			}

			if commonTerm == "X" {
				commonTerm = routeTerm(c.routeInfo)
			} else if commonTerm != routeTerm(c.routeInfo) {
				commonTerm = "X"
				break
			}
		}
		if commonTerm != "X" {
			addTerm = false
			sb.WriteString("(?:")
		}

		if n.factorChar != 0 {
			sb.WriteString("(?:(")
			regexEscape(string(n.factorChar), &sb)
			sb.WriteString(")(?:")
			i := 0
			for _, c := range n.children {
				if c == nil {
					continue
				}
				if i != 0 {
					sb.WriteByte('|')
				}
				rec(c, addTerm)
				i++
			}
			sb.WriteString("))")
		} else {
			cpre := n.routeInfo.Compiled.ConstantPortionRegexp(n.leftOffset)
			sb.WriteString(cpre)
			if len(n.children) == 0 {
				if n.routeInfo.Info.Terminal && addTerm {
					sb.WriteString(routeTerm(n.routeInfo))
				}
			} else {
				sb.WriteString("(?:")
				if n.routeInfo.Info.Terminal && addTerm {
					sb.WriteString(routeTerm(n.routeInfo))
					sb.WriteByte('|')
				}
				if !isJustSlash(n.routeInfo) {
					sb.WriteString("(\\/)\\/*")
				}
				sb.WriteString("(?:")
				i := 0
				for _, c := range n.children {
					if c == nil {
						continue
					}
					if i != 0 {
						sb.WriteByte('|')
					}
					rec(c, addTerm)
					i++
				}
				sb.WriteString("))")
			}
		}

		if commonTerm != "X" {
			sb.WriteByte(')')
			sb.WriteString(commonTerm)
		}
	}

	for i, c := range tree.children {
		if c == nil {
			continue
		}
		if i != 0 {
			sb.WriteByte('|')
		}
		rec(c, true)
	}

	sb.WriteString("))")

	return sb.String()
}

func getFirstChar(ri *CompiledRoute, leftOffset int) rune {
	if len(ri.Compiled.Elems) == 0 || ri.Compiled.Elems[0].kind != constant {
		return 0
	}

	start := ri.Compiled.Elems[0].value
	if leftOffset >= len(start) {
		return 0
	}
	r, sz := utf8.DecodeRuneInString(start[leftOffset:])
	if leftOffset+sz > len(start) {
		panic("Internal error in 'getFirstChar'")
	}
	return r
}

func routeTerm(r *CompiledRoute) string {
	if len(r.Compiled.Elems) > 0 {
		if r.Compiled.Elems[len(r.Compiled.Elems)-1].kind == slash {
			return "\\/+"
		}
		if r.Compiled.Elems[len(r.Compiled.Elems)-1].kind == noTrailingSlash {
			return ""
		}
	}
	return "\\/*"
}

func isJustSlash(r *CompiledRoute) bool {
	return len(r.Compiled.Elems) == 0
}

func wrapConstantPortionRegexp(re string) string {
	return "^" + re + "(?:\\?[^#]*)?(?:#.*)?$"
}

func wrapMatchRegexp(re string) string {
	return "^" + re + "(\\?[^#]*)?(#.*)?$"
}

func getNCaptureGroups(re string) int {
	n := 0
	for i := range re {
		c := re[i]
		if c == '(' && (i == 0 || re[i-1] != '\\') && (i+2 >= len(re) || re[i+1] != '?' || re[i+2] != ':') {
			n++
		}
	}
	return n
}

//nolint:unused // unused function used for debugging
func debugPrintCpTree(n *cpNode, indent int) string {
	if n == nil {
		return ""
	}
	out := ""
	for i := 0; i < indent; i++ {
		out += "    "
	}
	var cpre string
	if n.factorChar != 0 {
		cpre = fmt.Sprintf("%v:", string(n.factorChar))
	} else if n.routeInfo != nil {
		cpre = n.routeInfo.Compiled.ConstantPortionRegexp(n.leftOffset)
		if cpre == "" {
			cpre = "\"\""
		}
	} else {
		cpre = "."
	}
	out += cpre + "\n"
	for _, c := range n.children {
		out += debugPrintCpTree(c, indent+1)
	}
	return out
}

//nolint:unused // unused function used for debugging
func debugFormatRouteRegexps(rr *routeRegexps) string {
	out := make([]byte, 0)
	out = append(out, "constantPortionRegexp="...)
	out = appendJsonString(out, rr.constantPortionRegexp)
	out = append(out, "\nconstantPortionNGroups="...)
	out = append(out, fmt.Sprintf("%v", rr.constantPortionNGroups)...)
	out = append(out, "\nfamilies=\n<families>\n"...)
	for i := range rr.families {
		if i != 0 {
			out = append(out, '\n')
		}
		out = append(out, debugFormatRouteFamily(&rr.families[i])...)
	}
	out = append(out, "\n</families>"...)
	return string(out)
}

//nolint:unused // unused function used for debugging
func debugFormatRouteInfo(ri *CompiledRoute) string {
	out := make([]byte, 0)
	out = append(out, "name="...)
	out = appendJsonString(out, ri.Info.Name)
	out = append(out, "\nelems="...)
	out = append(out, debugPrintParsedRoute(ri.Compiled.Elems)...)
	out = append(out, "\nmatchRegexp="...)
	out = appendJsonString(out, ri.Compiled.MatchRegexp)
	out = append(out, "\nconstantPortionRegexp="...)
	out = appendJsonString(out, ri.Compiled.ConstantPortionRegexp(0))
	out = append(out, "\nconstantPortion="...)
	out = appendJsonString(out, ri.Compiled.ConstantPortion)
	out = append(out, "\nconstishPrefix="...)
	out = appendJsonString(out, ri.Compiled.ConstishPrefix)
	out = append(out, "\nnGroups="...)
	out = append(out, fmt.Sprintf("%v", ri.Compiled.NGroups)...)
	out = append(out, "\nparamGroupNumbers="...)
	out = append(out, fmt.Sprintf("%+v", debugFormatParamGroupNumbers(ri.Compiled.ParamGroupNumbers))...)
	out = append(out, "\ntags="...)
	out = append(out, fmt.Sprintf("%+v", stringSetToList(ri.Info.Tags))...)
	out = append(out, "\nmethods="...)
	out = append(out, fmt.Sprintf("%+v", stringSetToList(ri.Info.Methods))...)
	out = append(out, "\ndepth="...)
	out = append(out, fmt.Sprintf("%v", ri.Info.Depth)...)
	out = append(out, "\nterminal="...)
	out = append(out, fmt.Sprintf("%v", ri.Info.Terminal)...)
	out = append(out, "\nline="...)
	out = append(out, fmt.Sprintf("%v", ri.Info.Line)...)
	return string(out)
}

//nolint:unused // unused function used for debugging
func debugFormatRouteFamily(rf *routeFamily) string {
	out := make([]byte, 0)
	out = append(out, "constantPortion="...)
	out = appendJsonString(out, rf.constantPortion)
	out = append(out, "\nmembers=\n<route-group-members>"...)
	for i := range rf.members {
		out = append(out, debugFormatGroupMember(&rf.members[i])...)
	}
	out = append(out, "\n</route-group-members>\nnonparamGroupNumbers"...)
	out = append(out, fmt.Sprintf("%v", rf.nonparamGroupNumbers)...)
	out = append(out, "\nnLevels"...)
	out = append(out, fmt.Sprintf("%v", rf.nLevels)...)
	out = append(out, "\nmatchRegexp="...)
	out = appendJsonString(out, rf.matchRegexp)
	return string(out)
}

//nolint:unused // unused function used for debugging
func debugFormatGroupMember(rgm *routeGroupMember) string {
	out := make([]byte, 0)
	out = append(out, "name="...)
	out = appendJsonString(out, rgm.name)
	out = append(out, "\nroute=\n<route-with-parents>\n"...)
	out = append(out, debugFormatRouteWithParents(rgm.route)...)
	out = append(out, "\n</route-with-parents>\nparamGroupNumbers="...)
	out = append(out, fmt.Sprintf("%+v", debugFormatParamGroupNumbers(rgm.paramGroupNumbers))...)
	return string(out)
}

//nolint:unused // unused function used for debugging
func debugFormatRouteWithParents(rwp *RouteWithParents) string {
	return debugFormatRouteInfo(rwp.Route)
}

//nolint:unused // unused function used for debugging
func debugFormatParamGroupNumbers(pgns map[string]int) string {
	keys := make([]string, 0)
	for k := range pgns {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for i, k := range keys {
		if i != 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%v: %v", k, pgns[k]))
	}
	return sb.String()
}
