package compiler

import (
	"fmt"
	"sort"
	"strings"

	"github.com/addrummond/claney/glob"
)

type RouteInfo struct {
	Name                   string
	Line                   int
	Filename               string
	elems                  []routeElement
	matchRegexp            string
	constantPortionRegexp  string
	constantPortion        string
	constishPrefix         string // constant bar allowance of repeated '/', etc.
	constishSuffix         string
	nGroups                int
	constantPortionNGroups int
	paramGroupNumbers      map[string]int
	tags                   map[string]struct{}
	depth                  int
	terminal               bool
	methods                map[string]struct{}
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
	Route   *RouteInfo
	Parents []*RouteInfo
}

func routeToRegexps(elems []routeElement) RouteInfo {
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
			cp.WriteByte('(')
			regexEscape(elem.value, &cp)
			cp.WriteByte(')')
			constantPortionNGroups++
			constantPortion.WriteString(elem.value)
			constantPortionI++
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

	return RouteInfo{
		elems:                  elems,
		constantPortion:        constantPortion.String(),
		matchRegexp:            re.String(),
		constantPortionRegexp:  cp.String(),
		constishPrefix:         constishPrefix.String(),
		constishSuffix:         constishSuffix.String(),
		nGroups:                groupI - 1,
		constantPortionNGroups: constantPortionNGroups,
		paramGroupNumbers:      paramGroupNumbers,
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

func ProcessRouteFile(files [][]RouteFileEntry, filenames []string, nameSeparator string, groupObserver func([]RouteWithParents)) ([]RouteInfo, []RouteError) {
	if len(files) != len(filenames) {
		panic("Error in 'ProcessRouteFile': files and filenames args must be of the same length")
	}

	infos := make([]RouteInfo, 0)
	errors := make([]RouteError, 0)

	type tne struct {
		file      string
		line      int
		fileIndex int
		entryInex int
	}

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

			ri := routeToRegexps(entry.pattern)
			ri.Name = name
			ri.depth = len(levels)
			ri.Line = entry.line
			ri.Filename = filenames[fi]
			ri.tags = entry.tags
			ri.methods = entry.methods
			ri.terminal = entry.terminal

			levels = append(levels, level{entry.name, entry.pattern, entry.indent})

			infos = append(infos, ri)
		}
	}

	rwps := getRoutesWithParents(infos)

	terminals := make([]RouteWithParents, 0)
	for _, r := range rwps {
		if r.Route.terminal {
			terminals = append(terminals, r)
		}
	}

	grouped := groupRoutes(terminals)
	for _, g := range grouped {
		if len(g) <= biggestOverlapGroupAllowedBeforeWarning {
			continue
		}

		groupObserver(g)
	}

	overlaps := checkForOverlaps(grouped)
	for _, o := range overlaps {
		errors = append(errors, RouteError{OverlappingRoutes, o.route1.Line, "", o.route2.Line, nil, []string{o.route1.Filename, o.route2.Filename}})
	}

	// Check for any terminal routes with the same name that aren't adjacent in the file.
	for name, lines := range terminalLines {
		// No uniqueness checks for JSON route names
		if isJsonRouteName(name) {
			continue
		}

		if len(lines) <= 1 {
			continue
		}

		sort.Slice(lines, func(i, j int) bool { return lines[i].line < lines[j].line })
		for i := 0; i < len(lines)-1; i++ {
			if lines[i].file != lines[i+1].file {
				errors = append(errors, RouteError{DuplicateRouteName, lines[i].line, name, lines[i+1].line, nil, []string{lines[i].file, lines[i+1].file}})
				break
			}
			for l := lines[i].line + 1; l < lines[i+1].line; l++ {
				if _, ok := linesWithEntries[l]; ok {
					errors = append(errors, RouteError{DuplicateRouteName, lines[i].line, name, lines[i+1].line, nil, []string{lines[i].file}})
					break
				}
			}
		}
	}

	return infos, errors
}

type overlapBetween struct {
	route1, route2 *RouteInfo
}

const biggestOverlapGroupAllowedBeforeWarning = 5
const maxOverlaps = 10

func groupRoutes(rwps []RouteWithParents) [][]RouteWithParents {
	byPrefix := groupByConstishPrefix(rwps)
	byPrefixAndSuffix := make([][]RouteWithParents, 0)
	for _, routes := range byPrefix {
		byPrefixAndSuffix = append(byPrefixAndSuffix, groupByConstishSuffix(routes)...)
	}
	return byPrefixAndSuffix
}

func checkForOverlaps(grouped [][]RouteWithParents) []overlapBetween {
	overlaps := make([]overlapBetween, 0)
	for _, routes := range grouped {
		overlaps = append(overlaps, checkForOverlapsWithinGroup(routes)...)
		if len(overlaps) > maxOverlaps {
			break
		}
	}

	return overlaps
}

func checkForOverlapsWithinGroup(rwps []RouteWithParents) []overlapBetween {
	regexps := make([]*node, 0)
	regexpToInfo := make(map[*node]*RouteInfo)

	for _, rwp := range rwps {
		var resb strings.Builder
		resb.WriteString("\\/+")
		for i, p := range rwp.Parents {
			if i != 0 {
				resb.WriteString("\\/+")
			}
			resb.WriteString(p.constantPortionRegexp)
		}
		if len(rwp.Parents) > 0 {
			resb.WriteString("\\/+")
		}
		resb.WriteString(rwp.Route.constantPortionRegexp)
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
		for m1 := range ri1.methods {
			if _, ok := ri2.methods[m1]; ok {
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

func getRoutesWithParents(routes []RouteInfo) []RouteWithParents {
	rwps := make([]RouteWithParents, len(routes))

	i := 0
	withParentRoutes(routes, func(r *RouteInfo, parents []*RouteInfo) {
		rwps[i] = RouteWithParents{r, parents}
		i++
	})

	return rwps
}

func withParentRoutes(routes []RouteInfo, iter func(*RouteInfo, []*RouteInfo)) {
	lastLevel := 0
	parentRoutes := make([]*RouteInfo, 0)

	for i := range routes {
		r := &routes[i]

		if r.depth > lastLevel && i > 0 {
			parentRoutes = append(parentRoutes, &routes[i-1])
		} else if r.depth < lastLevel {
			parentRoutes = parentRoutes[0 : len(parentRoutes)-(lastLevel-r.depth)]
		}

		cp := make([]*RouteInfo, len(parentRoutes))
		copy(cp, parentRoutes)
		iter(r, cp)

		lastLevel = r.depth
	}
}

type familyWithConstantPortion struct {
	constantPortion string
	routes          []RouteWithParents
}

func familiesByConstantPortion(routes []RouteInfo) []familyWithConstantPortion {
	families := make(map[string][]RouteWithParents, 0)
	constantPortionUpTo := make(map[*RouteInfo]string)

	withParentRoutes(routes, func(r *RouteInfo, parents []*RouteInfo) {
		var cpb strings.Builder
		for i, p := range parents {
			if i != 0 && !isJustSlash(parents[i-1]) {
				cpb.WriteString("/")
			}
			cpb.WriteString(p.constantPortion)
		}
		if len(parents) != 0 && !isJustSlash(parents[len(parents)-1]) {
			cpb.WriteString("/")
		}
		cpb.WriteString(r.constantPortion)
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

func GetRouteRegexps(routes []RouteInfo) routeRegexps {
	constantPortionRegexp, extraCaptureGroups := getConstantPortionRegexp(routes)

	byCp := familiesByConstantPortion(routes)
	families := make([]routeFamily, 0)

	totalNGroups := extraCaptureGroups
	for _, r := range routes {
		totalNGroups += r.constantPortionNGroups
	}

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
		if r.Route.terminal {
			terms = append(terms, &rs[i])
		}
	}
	return terms
}

type IncludeSpec struct {
	Include bool
	// One of these is "", the other is not
	TagGlob string
	Method  string
}

func RouteRegexpsToJSON(rrs *routeRegexps, tagGlobs []IncludeSpec) ([]byte, int) {
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
		if !includeFamily(&g, tagGlobs) {
			continue
		}

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
			if !matchingSpec(tagGlobs, m.route.Route.methods, m.route.Route.tags) {
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
			for k, m := range stringSetToList(m.route.Route.methods) {
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

func includeFamily(family *routeFamily, specs []IncludeSpec) bool {
	for _, m := range family.members {
		if matchingSpec(specs, m.route.Route.methods, m.route.Route.tags) {
			return true
		}
	}
	return false
}

func matchingSpec(specs []IncludeSpec, methods map[string]struct{}, tags map[string]struct{}) bool {
	if len(specs) == 0 {
		return true
	}

	// Include by default if first in sequence is exclude, or exclude by default
	// if first in sequence is include.
	included := !specs[0].Include

	removedMethods := make(map[string]struct{})

	for _, s := range specs {
		if s.TagGlob != "" {
			for t := range tags {
				if glob.Glob(s.TagGlob, t) {
					included = s.Include
					break
				}
			}
		} else if s.Method != "" {
			ucmeth := strings.ToUpper(s.Method)
			if _, ok := methods[ucmeth]; ok {
				if s.Include {
					included = true
					delete(removedMethods, ucmeth)
				} else {
					removedMethods[ucmeth] = struct{}{}
					included = len(removedMethods) != len(methods)
				}
			} else {
				included = !s.Include
			}
		} else {
			panic("Internal error: Bad IncludeSpec")
		}
	}
	return included
}

func computeTags(m *routeGroupMember) []string {
	tags := make(map[string]struct{}, len(m.route.Route.tags)*2)
	for k := range m.route.Route.tags {
		tags[k] = struct{}{}
	}
	for _, p := range m.route.Parents {
		for k := range p.tags {
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

			sb.WriteString(p.matchRegexp)
		}
		if len(r.Parents) > 0 && !isJustSlash(r.Parents[len(r.Parents)-1]) {
			sb.WriteString("\\/+")
		}
		sb.WriteString(r.Route.matchRegexp)
		sb.WriteString(routeTerm(r.Route))

		names[i] = r.Route.Name

		paramGroups[i] = make(map[string]int)
		for _, p := range r.Parents {
			for k, v := range p.paramGroupNumbers {
				paramGroups[i][k] = currentGroupNumber + v - 1 // groups numbered from 1
			}
			currentGroupNumber += p.nGroups
		}

		for k, v := range r.Route.paramGroupNumbers {
			paramGroups[i][k] = currentGroupNumber + v - 1 // groups numbered from 1
		}
		currentGroupNumber += r.Route.nGroups

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

func getConstantPortionRegexp(routes []RouteInfo) (string, int) {
	var sb strings.Builder
	sb.WriteString("(?:")

	lastLevel := 0
	extraCaptureGroups := 0
	parentRoutes := make([]*RouteInfo, 0)
	for i := range routes {
		r := &routes[i]

		regexp := r.constantPortionRegexp

		if r.depth > lastLevel {
			parentRoutes = append(parentRoutes, &routes[i-1])
			sb.WriteString("(?:")
			if routes[i-1].terminal {
				sb.WriteString(routeTerm(&routes[i-1]))
				sb.WriteString("|")
			}
		} else if r.depth < lastLevel {
			parentRoutes = parentRoutes[0 : len(parentRoutes)-(lastLevel-r.depth)]
			for j := 0; j < lastLevel-r.depth; j++ {
				sb.WriteByte(')')
			}
		}

		if i != 0 && routes[i-1].depth >= r.depth {
			sb.WriteString("|")
		}
		sb.WriteString("(?:")
		if len(parentRoutes) > 0 && !isJustSlash(parentRoutes[len(parentRoutes)-1]) {
			sb.WriteString("(\\/)\\/*")
			extraCaptureGroups++
		} else if len(parentRoutes) == 0 {
			sb.WriteString("\\/+")
		}
		sb.WriteString(regexp)
		if r.terminal && (i+1 >= len(routes) || routes[i+1].depth <= r.depth) {
			sb.WriteString(routeTerm(r))
		}
		sb.WriteByte(')')

		lastLevel = r.depth
	}

	for j := 0; j < lastLevel; j++ {
		sb.WriteByte(')')
	}

	sb.WriteString(")")

	return sb.String(), extraCaptureGroups
}

func routeTerm(r *RouteInfo) string {
	if len(r.elems) > 0 {
		if r.elems[len(r.elems)-1].kind == slash {
			return "\\/+"
		}
		if r.elems[len(r.elems)-1].kind == noTrailingSlash {
			return ""
		}
	}
	return "\\/*"
}

func isJustSlash(r *RouteInfo) bool {
	return len(r.elems) == 0
}

func wrapConstantPortionRegexp(re string) string {
	return "^" + re + "(?:\\?[^#]*)?(?:#.*)?$"
}

func wrapMatchRegexp(re string) string {
	return "^" + re + "(\\?[^#]*)?(#.*)?$"
}

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

func debugFormatRouteInfo(ri *RouteInfo) string {
	out := make([]byte, 0)
	out = append(out, "name="...)
	out = appendJsonString(out, ri.Name)
	out = append(out, "\nelems="...)
	out = append(out, debugPrintParsedRoute(ri.elems)...)
	out = append(out, "\nmatchRegexp="...)
	out = appendJsonString(out, ri.matchRegexp)
	out = append(out, "\nconstantPortionRegexp="...)
	out = appendJsonString(out, ri.constantPortionRegexp)
	out = append(out, "\nconstantPortion="...)
	out = appendJsonString(out, ri.constantPortion)
	out = append(out, "\nconstishPrefix="...)
	out = appendJsonString(out, ri.constishPrefix)
	out = append(out, "\nnGroups="...)
	out = append(out, fmt.Sprintf("%v", ri.nGroups)...)
	out = append(out, "\nconstantPortionNGroups="...)
	out = append(out, fmt.Sprintf("%v", ri.constantPortionNGroups)...)
	out = append(out, "\nparamGroupNumbers="...)
	out = append(out, fmt.Sprintf("%+v", debugFormatParamGroupNumbers(ri.paramGroupNumbers))...)
	out = append(out, "\ntags="...)
	out = append(out, fmt.Sprintf("%+v", stringSetToList(ri.tags))...)
	out = append(out, "\nmethods="...)
	out = append(out, fmt.Sprintf("%+v", stringSetToList(ri.methods))...)
	out = append(out, "\ndepth="...)
	out = append(out, fmt.Sprintf("%v", ri.depth)...)
	out = append(out, "\nterminal="...)
	out = append(out, fmt.Sprintf("%v", ri.terminal)...)
	out = append(out, "\nline="...)
	out = append(out, fmt.Sprintf("%v", ri.Line)...)
	return string(out)
}

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

func debugFormatRouteWithParents(rwp *RouteWithParents) string {
	return debugFormatRouteInfo(rwp.Route)
}

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
