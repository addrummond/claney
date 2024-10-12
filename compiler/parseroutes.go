package compiler

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

type routeElementKind int

const (
	illegalCodePoint       routeElementKind = iota
	illegalQuestionMark    routeElementKind = iota
	illegalHash            routeElementKind = iota
	illegalWhitespace      routeElementKind = iota
	illegalCharInParamName routeElementKind = iota
	illegalBackslashEscape routeElementKind = iota
	slash                  routeElementKind = iota
	constant               routeElementKind = iota
	parameter              routeElementKind = iota
	integerParameter       routeElementKind = iota
	restParameter          routeElementKind = iota
	singleGlob             routeElementKind = iota
	doubleGlob             routeElementKind = iota
	noTrailingSlash        routeElementKind = iota
)

func (k routeElementKind) String() string {
	switch k {
	case illegalCodePoint:
		return "<illegal-code-point>"
	case illegalQuestionMark:
		return "?"
	case illegalHash:
		return "#"
	case illegalWhitespace:
		return "<illegal-whitespace>"
	case illegalCharInParamName:
		return "<illegal-param-name-char>"
	case illegalBackslashEscape:
		return "<illegal-backslash-escape>"
	case slash:
		return "/"
	case constant:
		return "const"
	case parameter:
		return "param"
	case integerParameter:
		return "integerParam"
	case restParameter:
		return "restParam"
	case singleGlob:
		return "*"
	case doubleGlob:
		return "**"
	case noTrailingSlash:
		return "!/"
	}
	panic(fmt.Sprintf("Unrecognized routeElementKind %v", int(k)))
}

type routeElement struct {
	kind  routeElementKind
	value string
	col   int
}

func badCodePoint(r rune) bool {
	return r == 0 || (unicode.IsControl(r) && r != '\t' && r != '\n' && r != '\r')
}

type CasePolicy int

const (
	AllowUpperCase    CasePolicy = iota
	DisallowUpperCase CasePolicy = iota
)

func parseRoute(route string) []routeElement {
	elems := make([]routeElement, 0)

	// all special chars are ASCII so we can iterate by byte
	currentElem := routeElement{}
	for i := 0; i < len(route); {
		b := route[i]

		currentElem = routeElement{}
		currentElem.col = i
		startI := i
		switch b {
		case '\x00', '\x01', '\x02', '\x03', '\x04', '\x05', '\x06', '\x07', '\x08', '\x0B', '\x0C', '\x0E', '\x0F', '\x10', '\x11', '\x12', '\x13', '\x14', '\x15', '\x16', '\x17', '\x18', '\x19', '\x1A', '\x1B', '\x1C', '\x1D', '\x1E', '\x1F':
			i++
			currentElem.kind = illegalCodePoint
		case '?':
			i++
			currentElem.kind = illegalQuestionMark
		case '#':
			i++
			currentElem.kind = illegalHash
		case '/':
			i++
			currentElem.kind = slash
		case '!':
			if i == len(route)-2 && route[i+1] == '/' {
				i += 2
				currentElem.kind = noTrailingSlash
			} else {
				i++
				currentElem.kind = constant
				currentElem.value = "!"
			}
		case '*':
			if i+1 < len(route) && route[i+1] == '*' {
				i += 2
				currentElem.kind = doubleGlob
			} else {
				i++
				currentElem.kind = singleGlob
			}
		case ':':
			isInteger := false
			isRest := false
			if i+1 < len(route) && route[i+1] == '#' {
				isInteger = true
				i++
			} else if i+2 < len(route) && route[i+1] == '*' && route[i+2] == '*' {
				isRest = true
				i += 2
			}

			var badEscape bool
			var sb strings.Builder
			nameStartI := i
			i++
			if nameStartI+1 < len(route) && route[nameStartI+1] == '{' {
				i++
				if route[nameStartI+1] != '{' {
					i += 2
				}

				badChar := false
				for i < len(route) {
					if route[i] == '}' {
						i++
						break
					}
					if route[i] == '\\' && i+1 < len(route) {
						i++
						if route[i] != '\\' && route[i] != '}' && route[i] != '#' {
							badEscape = true
						}
					}
					r, l := utf8.DecodeRuneInString(route[i:])
					if r == 0 || (unicode.IsSpace(r) && r != ' ') || badCodePoint(r) {
						badChar = true
					}
					sb.WriteRune(r)
					i += l
				}
				if sb.Len() == 0 {
					currentElem.kind = constant
					currentElem.value = ":{}"
				} else {
					currentElem.kind = parameter
					if isInteger {
						currentElem.kind = integerParameter
					} else if isRest {
						currentElem.kind = restParameter
					}
					currentElem.value = sb.String()
				}
				if badChar {
					elems = append(elems, routeElement{illegalCharInParamName, "", startI})
				}
			} else {
				for i < len(route) {
					if route[i] == '\\' && i+1 < len(route) {
						i++
						if route[i] != '\\' && route[i] != ':' && route[i] != '#' {
							badEscape = true
						}
						sb.WriteByte(route[i])
						i++
					} else {
						r, l := utf8.DecodeRuneInString(route[i:])
						if r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r) {
							sb.WriteRune(r)
							i += l
						} else {
							break
						}
					}
				}
				if sb.Len() == 0 {
					currentElem.kind = constant
					currentElem.value = route[startI:i]
				} else {
					currentElem.kind = parameter
					if isInteger {
						currentElem.kind = integerParameter
					} else if isRest {
						currentElem.kind = restParameter
					}
					currentElem.value = sb.String()
				}
			}
			if badEscape {
				elems = append(elems, routeElement{illegalBackslashEscape, "", i})
			}
		default:
			currentElem.kind = constant
			var sb strings.Builder
			for i < len(route) && route[i] != '/' && route[i] != '!' && route[i] != '*' && route[i] != ':' && route[i] != '?' && route[i] != '#' {
				if route[i] == '\\' {
					i++
					if i == len(route) {
						sb.WriteByte('\\')
					} else if route[i] == ':' || route[i] == '!' || route[i] == '[' || route[i] == ']' || route[i] == '*' {
						sb.WriteByte(route[i])
						i++
					} else {
						currentElem.value = sb.String()
						elems = append(elems, currentElem)
						elems = append(elems, routeElement{illegalBackslashEscape, "", i})
						currentElem.kind = constant
						currentElem.value = string(route[i])
						sb.Reset()
						i++
					}
				} else {
					r, l := utf8.DecodeRuneInString(route[i:])
					if unicode.IsSpace(r) {
						currentElem.value = sb.String()
						elems = append(elems, currentElem)
						elems = append(elems, routeElement{illegalWhitespace, "", i})
						currentElem.kind = constant
						currentElem.value = ""
						sb.Reset()
					} else if badCodePoint(r) {
						currentElem.value = sb.String()
						elems = append(elems, currentElem)
						elems = append(elems, routeElement{illegalCodePoint, "", i})
						currentElem.kind = constant
						currentElem.value = ""
						sb.Reset()
					} else {
						sb.WriteString(route[i : i+l])
					}
					i += l
				}
			}
			currentElem.value = sb.String()
		}

		elems = append(elems, currentElem)
	}
	return elems
}

func validateRouteElems(initialIndent, indent int, elems []routeElement) []RouteErrorKind {
	if len(elems) == 0 {
		return []RouteErrorKind{MissingNameOrRoute}
	}

	var errors []RouteErrorKind

	for i := 0; i < len(elems)-1; i++ {
		if elems[i].kind == slash && elems[i+1].kind == slash {
			errors = append(errors, MultipleSlashesInARow)
			break
		}
	}

	if (initialIndent == -1 || indent == initialIndent) && elems[0].kind != slash {
		errors = append(errors, RootMustStartWithSlash)
	}
	if elems[len(elems)-1].kind == noTrailingSlash {
		if len(elems) == 1 {
			errors = append(errors, OnlyNoTrailingSlash)
		} else if elems[len(elems)-1].kind == slash {
			errors = append(errors, NoTrailingSlashAfterSlash)
		}
	}

	return errors
}

type RouteFileEntry struct {
	indent   int
	name     string
	pattern  []routeElement
	line     int
	terminal bool // if false, the route exists only as a parent of other routes
	tags     map[string]struct{}
	methods  map[string]struct{}
}

type RouteErrorKind int

const RouteWarning RouteErrorKind = (1 << 30)

const (
	MissingNameOrRoute RouteErrorKind = iota
	DuplicateRouteName
	RootMustStartWithSlash
	OverlappingRoutes
	MisplacedDot
	RouteContainsBadCodePoint
	QuestionMarkInRoute
	HashInRoute
	WhitespaceInRoute
	IllegalCharInParamName
	IllegalBackslashEscape
	IllegalBackslashEscapeInRouteName
	NontabspaceIndentationCharacter
	BadCharacterInMethodName
	MissingCommaBetweenMethodNames
	TwoCommasInSequenceInMethodNames
	IndentLessThanFirstLine
	OnlyNoTrailingSlash
	NoTrailingSlashAfterSlash
	MultipleSlashesInARow
	UpperCaseCharInRoute
	IOError
	EmptyMethodList
	ExpectedJSONRoutesToBeArray
	ExpectedJSONRouteFileEntryToBeObject
	UnexpectedKeyInJSONRouteFile
	JSONRouteMissingNameField
	JSONRouteMissingPatternField
	// TODO rename below to have JSON in names
	UnexpectedTokenInJSONRouteFile
	NoSlashInsideJSONRoutePatternElement
	FirstMemberOfPatternElementMustBeString
	BadFirstMemberOfPatternElement
	UnexpectedPatternElementMember
	ParameterNameMustBeString
	WarningBigGroup = iota | RouteWarning
)

type RouteError struct {
	Kind          RouteErrorKind
	Line          int
	Col           int
	DuplicateName string
	OtherLine     int
	IOError       error
	Filenames     []string
	Group         []RouteWithParents
}

func (e RouteError) Error() string {
	var desc string
	switch e.Kind {
	case MissingNameOrRoute:
		desc = "missing route name or missing route pattern"
	case DuplicateRouteName:
		desc = fmt.Sprintf("two non-adjacently-nestled routes have the same name ('%v'); move them next to each other in the same file", e.DuplicateName)
	case RootMustStartWithSlash:
		desc = "pattern at root level must start with '/'"
	case OverlappingRoutes:
		desc = "routes overlap"
	case MisplacedDot:
		desc = "misplaced '.': should come immediately after parent route and be indented under it"
	case RouteContainsBadCodePoint:
		desc = "route contains bad code point"
	case NontabspaceIndentationCharacter:
		desc = "route is indented with a whitespace character other than a tab or a space"
	case QuestionMarkInRoute:
		desc = "route may not contain '?'"
	case HashInRoute:
		desc = "route may not contain '#'"
	case WhitespaceInRoute:
		desc = "route may not contain whitespace"
	case IllegalBackslashEscape:
		desc = "illegal backslash escape"
	case IllegalBackslashEscapeInRouteName:
		desc = "illegal backslash escape in route name"
	case BadCharacterInMethodName:
		desc = "bad character in method name"
	case MissingCommaBetweenMethodNames:
		desc = "missing comma between method names"
	case TwoCommasInSequenceInMethodNames:
		desc = "two commas in sequence in list of method names"
	case IndentLessThanFirstLine:
		desc = "the line is indented less than the first non-blank line of the input file"
	case OnlyNoTrailingSlash:
		desc = "the route consists entirely of a '!/' prohibition on trailing slashes"
	case NoTrailingSlashAfterSlash:
		desc = "the '!/' sequence banning trailing slashes follows a slash"
	case MultipleSlashesInARow:
		desc = "multiple slashes in a row in route"
	case UpperCaseCharInRoute:
		desc = "upper case character in route"
	case IOError:
		desc = fmt.Sprintf("IO Error: %v", e.IOError)
	case EmptyMethodList:
		desc = "Empty method list"
	case ExpectedJSONRoutesToBeArray:
		desc = "Expected JSON route file to be array"
	case ExpectedJSONRouteFileEntryToBeObject:
		desc = "Expected route file entry to be object in JSON route file"
	case UnexpectedKeyInJSONRouteFile:
		desc = "Unexpected key or key value type in JSON route file"
	case JSONRouteMissingNameField:
		desc = "Route missing name field in JSON route file"
	case JSONRouteMissingPatternField:
		desc = "Route missing pattern field in JSON route file"
	case UnexpectedTokenInJSONRouteFile:
		desc = "Unexpected token in JSON route file"
	case NoSlashInsideJSONRoutePatternElement:
		desc = "No '/' allowed inside JSON route pattern element"
	case FirstMemberOfPatternElementMustBeString:
		desc = "First member of pattern element must be string"
	case BadFirstMemberOfPatternElement:
		desc = "Bad first member of pattern element"
	case UnexpectedPatternElementMember:
		desc = "Unexpected pattern element member"
	case ParameterNameMustBeString:
		desc = "Parameter name must be string"
	case WarningBigGroup:
		desc = "Big group"
	default:
		panic(fmt.Sprintf("unrecognized routeRrrorKind %v", int(e.Kind)))
	}

	return formatErrorMessage(e, desc)
}

func formatErrorMessage(e RouteError, desc string) string {
	var msg string

	if e.OtherLine == 0 {
		if e.Col != -1 {
			msg = fmt.Sprintf("%v:%v: %v", e.Line, e.Col, desc)
		} else {
			msg = fmt.Sprintf("%v: %v", e.Line, desc)
		}
		if len(e.Filenames) == 0 || e.Filenames[0] == "" {
			msg = "stdin:" + msg
		} else {
			msg = e.Filenames[0] + ":" + msg
		}
	} else if e.OtherLine != 0 && len(e.Filenames) == 2 {
		msg = fmt.Sprintf("%v:%v: (and %v:%v): %v", e.Filenames[0], e.Line, e.Filenames[1], e.OtherLine, desc)
	} else {
		// shouldn't get here
		msg = fmt.Sprintf("%v: (and %v): %v", e.Line, e.OtherLine, desc)
	}

	return msg
}

func ParseRouteFile(input io.Reader, casePolicy CasePolicy) ([]RouteFileEntry, []RouteError) {
	entries := make([]RouteFileEntry, 0)

	errors := make([]RouteError, 0)

	var currentLine strings.Builder
	scanner := bufio.NewScanner(input)
	sourceLine := 0
	firstSourceLineOfSplice := 0
	lineStarts := make([]int, 0)
	initialIndent := -1
	dotLevel := -1
	for scanner.Scan() {
		line := scanner.Text()
		sourceLine++

		lineStarts = append(lineStarts, currentLine.Len())

		if len(line) > 0 && line[len(line)-1] == '\\' && (len(line) == 1 || line[len(line)-2] != '\\') {
			currentLine.WriteString(line[:len(line)-1])
			firstSourceLineOfSplice = sourceLine
			continue
		}

		if currentLine.Len() == 0 {
			firstSourceLineOfSplice = sourceLine
			currentLine.WriteString(line)
		} else {
			// last line ended with '\', so strip leading whitespace
			stripped := stripLeadingWhitespace(line)
			currentLine.WriteString(stripped)
			lineStarts[len(lineStarts)-1] -= len(line) - len(stripped)
		}

		wholeLine := currentLine.String()
		currentLine.Reset()

		wholeLine = stripTrailingWhitespace(stripComment(wholeLine))

		if isBlank(wholeLine) {
			continue
		}

		indent := 0
		for i := 0; i < len(wholeLine); {
			nextr, sz := utf8.DecodeRuneInString(wholeLine[i:])
			if unicode.IsSpace(nextr) {
				if nextr != ' ' && nextr != '\t' {
					errors = append(errors, RouteError{NontabspaceIndentationCharacter, sourceLine, -1, "", 0, nil, nil, nil})
				}
				indent++
				i += sz
			} else if badCodePoint(nextr) {
				errors = append(errors, RouteError{IllegalBackslashEscapeInRouteName, sourceLine, -1, "", 0, nil, nil, nil})
				i += sz
			} else {
				break
			}
		}
		if indent < initialIndent {
			errors = append(errors, RouteError{IndentLessThanFirstLine, sourceLine, -1, "", 0, nil, nil, nil})
			continue
		}
		if initialIndent == -1 {
			initialIndent = indent
		}

		// Is it '.'?
		if isDot(wholeLine) {
			if len(entries) == 0 || entries[len(entries)-1].indent >= indent {
				errors = append(errors, RouteError{MisplacedDot, sourceLine, -1, "", 0, nil, nil, nil})
			}
			dotLevel = indent
			continue
		} else if indent < dotLevel {
			dotLevel = -1
		}

		if len(entries) != 0 && entries[len(entries)-1].indent < indent && dotLevel < indent {
			entries[len(entries)-1].terminal = false
		}

		var nameB strings.Builder
		i := indent
		for i < len(wholeLine) {
			if wholeLine[i] == '\\' {
				if i+1 < len(wholeLine) {
					rn, sz := utf8.DecodeRuneInString(wholeLine[i+1:])
					nameB.WriteRune(rn)
					i += sz + 1
					if badCodePoint(rn) {
						errors = append(errors, RouteError{RouteContainsBadCodePoint, sourceLine, -1, "", 0, nil, nil, nil})
					}
					if !unicode.IsSpace(rn) {
						errors = append(errors, RouteError{IllegalBackslashEscapeInRouteName, sourceLine, -1, "", 0, nil, nil, nil})
					}
				} else {
					errors = append(errors, RouteError{IllegalBackslashEscapeInRouteName, sourceLine, -1, "", 0, nil, nil, nil})
					i++
				}
			} else {
				rn, sz := utf8.DecodeRuneInString(wholeLine[i:])
				i += sz
				if unicode.IsSpace(rn) {
					break
				}
				if badCodePoint(rn) {
					errors = append(errors, RouteError{RouteContainsBadCodePoint, sourceLine, -1, "", 0, nil, nil, nil})
				} else {
					nameB.WriteRune(rn)
				}
			}
		}
		name := nameB.String()

		for i < len(wholeLine) {
			rn, sz := utf8.DecodeRuneInString(wholeLine[i:])
			if badCodePoint(rn) {
				errors = append(errors, RouteError{RouteContainsBadCodePoint, sourceLine, -1, "", 0, nil, nil, nil})
			}
			if !unicode.IsSpace(rn) {
				break
			}
			i += sz
		}

		if i >= len(wholeLine) {
			errors = append(errors, RouteError{MissingNameOrRoute, firstSourceLineOfSplice, -1, "", 0, nil, nil, nil})
			continue
		}

		methods := make(map[string]struct{}, 0)
		explicitMethodListPresent := false
		if wholeLine[i] == '[' {
			explicitMethodListPresent = true
			i++
			var currentMethod strings.Builder
			foundComma := false
			for i < len(wholeLine) {
				rn, sz := utf8.DecodeRuneInString(wholeLine[i:])
				i += sz
				if unicode.IsSpace(rn) || rn == ',' || rn == ']' {
					if currentMethod.Len() > 0 {
						methods[strings.ToUpper(currentMethod.String())] = struct{}{}
						currentMethod.Reset()
					}
					if rn == ',' {
						if foundComma {
							errors = append(errors, RouteError{TwoCommasInSequenceInMethodNames, sourceLine, -1, "", 0, nil, nil, nil})
						}
						foundComma = true
					}
				} else if badCodePoint(rn) {
					errors = append(errors, RouteError{RouteContainsBadCodePoint, sourceLine, -1, "", 0, nil, nil, nil})
				} else if (rn >= 'A' && rn <= 'Z') || (rn >= 'a' || rn <= 'z') { // ASCII letters only for method names
					if len(methods) > 0 && currentMethod.Len() == 0 && !foundComma {
						errors = append(errors, RouteError{MissingCommaBetweenMethodNames, sourceLine, -1, "", 0, nil, nil, nil})
					}
					foundComma = false
					currentMethod.WriteRune(rn)
				} else {
					errors = append(errors, RouteError{BadCharacterInMethodName, sourceLine, -1, "", 0, nil, nil, nil})
				}

				if rn == ']' {
					break
				}
			}
		}
		if len(methods) == 0 {
			if explicitMethodListPresent {
				errors = append(errors, RouteError{EmptyMethodList, sourceLine, -1, "", 0, nil, nil, nil})
			}
			methods["GET"] = struct{}{}
		}

		for i < len(wholeLine) {
			rn, sz := utf8.DecodeRuneInString(wholeLine[i:])
			if badCodePoint(rn) {
				errors = append(errors, RouteError{RouteContainsBadCodePoint, sourceLine, -1, "", 0, nil, nil, nil})
			}
			if !unicode.IsSpace(rn) {
				break
			}
			i += sz
		}

		patternString := wholeLine[i:]
		patternStart := i
		tags, tagsStart := getTags(patternString)
		patternString = patternString[0:tagsStart]

		pattern := parseRoute(patternString)

		validationErrorKinds := validateRouteElems(initialIndent, indent, pattern)
		if len(validationErrorKinds) > 0 {
			for _, k := range validationErrorKinds {
				errors = append(errors, RouteError{k, firstSourceLineOfSplice, -1, "", 0, nil, nil, nil})
			}
			continue
		}

		for _, elem := range pattern {
			switch elem.kind {
			case constant:
				if casePolicy == DisallowUpperCase {
					if lci := containsNonLowerCase(elem.value); lci != -1 {
						colZeroOffset := elem.col + lci + patternStart
						errors = append(errors, RouteError{UpperCaseCharInRoute, sourceLine, physicalLineColumn(lineStarts, colZeroOffset) + 1, "", 0, nil, nil, nil})
					}
				}
			case illegalCodePoint:
				errors = append(errors, RouteError{RouteContainsBadCodePoint, sourceLine, -1, "", 0, nil, nil, nil})
			case illegalQuestionMark:
				errors = append(errors, RouteError{QuestionMarkInRoute, sourceLine, -1, "", 0, nil, nil, nil})
			case illegalHash:
				errors = append(errors, RouteError{HashInRoute, sourceLine, -1, "", 0, nil, nil, nil})
			case illegalWhitespace:
				errors = append(errors, RouteError{WhitespaceInRoute, sourceLine, -1, "", 0, nil, nil, nil})
			case illegalCharInParamName:
				errors = append(errors, RouteError{IllegalCharInParamName, sourceLine, -1, "", 0, nil, nil, nil})
			case illegalBackslashEscape:
				errors = append(errors, RouteError{IllegalBackslashEscape, sourceLine, -1, "", 0, nil, nil, nil})
			}
		}

		notionalIndent := indent
		if indent == initialIndent {
			// Ensure initial indent is consistent across files
			notionalIndent = 0
		}

		entries = append(entries, RouteFileEntry{
			indent:   notionalIndent,
			name:     name,
			pattern:  pattern,
			line:     firstSourceLineOfSplice,
			terminal: true,
			tags:     tags,
			methods:  methods,
		})

		lineStarts = lineStarts[:0]
	}

	if err := scanner.Err(); err != nil {
		errors = append(errors, RouteError{IOError, sourceLine, -1, "", 0, nil, nil, nil})
	}

	return entries, errors
}

func physicalLineColumn(lineStarts []int, offset int) int {
	// Could use binary search here, but this is not a performance-critical path
	// (used only in error reporting), and very long splices should be uncommon.
	for i := len(lineStarts) - 1; i >= 0; i-- {
		if lineStarts[i] < offset {
			return offset - lineStarts[i]
		}
	}
	return 0
}

func ParseRouteFiles(inputFiles []string, inputReaders []io.Reader, casePolicy CasePolicy) ([][]RouteFileEntry, []RouteError) {
	if len(inputFiles) != len(inputReaders) {
		panic("Bad arguments passed to 'ParseRouteFiles': inputFiles and inputReaders must have same length")
	}

	entriesPerFile := make([][]RouteFileEntry, len(inputReaders))
	allErrors := make([][]RouteError, len(inputReaders))

	var wg sync.WaitGroup

	for i, r := range inputReaders {
		wg.Add(1)

		ii := i // for Go <= 1.21 compat
		rr := r // ditto

		go func() {
			defer wg.Done()
			ent, es := ParseRouteFile(rr, casePolicy)
			for j := range es {
				es[j].Filenames = []string{inputFiles[ii]}
			}
			entriesPerFile[ii] = ent
			allErrors[ii] = es
		}()
	}

	wg.Wait()

	return entriesPerFile, flatten(allErrors)
}

func stripLeadingWhitespace(line string) string {
	for i := 0; i < len(line); {
		rn, sz := utf8.DecodeRuneInString(line[i:])
		if !unicode.IsSpace(rn) {
			return line[i:]
		}
		i += sz
	}

	return ""
}

func stripTrailingWhitespace(line string) string {
	for i := len(line); i > 0; {
		rn, sz := utf8.DecodeLastRuneInString(line[0:i])
		if !unicode.IsSpace(rn) {
			return line[0:i]
		}
		i -= sz
	}

	return ""
}

func stripComment(line string) string {
	var b strings.Builder

	for i := range line {
		if line[i] == '#' && (i == 0 || line[i-1] != ':') {
			// there's a hash not preceded by a ':'
			if i > 0 && line[i-1] == '\\' && (i-2 < 0 || line[i-2] != '\\') {
				// it's escaped, so it's not a comment.
				if b.Len() == 0 {
					b.WriteString(line[0 : i-1])
				}
				b.WriteByte('#')
			} else {
				// it's not escaped, so it is the start of a comment
				if b.Len() > 0 {
					return b.String()
				}
				return line[0:i]
			}
		} else if b.Len() > 0 {
			// we can't use a subslice of the input string because of an escaped '#',
			// so copy the char to a new string.
			b.WriteByte(line[i])
		}
	}

	if b.Len() > 0 {
		return b.String()
	}
	return line
}

func getTags(routeString string) (map[string]struct{}, int) {
	hasTags := false
	var ti int
	for ti = len(routeString); ti >= 0; {
		rn, sz := utf8.DecodeLastRuneInString(routeString[0:ti])
		if rn == ']' && routeString[ti-1] != '\\' {
			hasTags = true
			ti--
			break
		}
		if !unicode.IsSpace(rn) {
			break
		}
		ti -= sz
	}

	if !hasTags {
		return map[string]struct{}{}, len(routeString)
	}

	ti--
	tags := make(map[string]struct{})
	currentTag := make([]byte, len(routeString))
	currentTagI := len(currentTag) - 1
	leftmostNonwhitespaceI := len(currentTag)
	rightmostNonwhitespaceI := len(currentTag)
	foundRightmostNonwhitespace := false
	for ti >= 0 {
		if routeString[ti] == ',' || routeString[ti] == '[' || routeString[ti] == ']' {
			if ti > 0 && routeString[ti-1] == '\\' {
				leftmostNonwhitespaceI = currentTagI
				if !foundRightmostNonwhitespace {
					rightmostNonwhitespaceI = currentTagI + 1
					foundRightmostNonwhitespace = true
				}
				currentTag[currentTagI] = routeString[ti]
				currentTagI--
				ti -= 2
			} else {
				tagStr := string(currentTag[leftmostNonwhitespaceI:rightmostNonwhitespaceI])

				if tagStr != "" {
					tags[tagStr] = struct{}{}
				}

				currentTagI = len(currentTag) - 1
				leftmostNonwhitespaceI = len(currentTag)
				rightmostNonwhitespaceI = len(currentTag)
				foundRightmostNonwhitespace = false
				if routeString[ti] == '[' {
					break
				}
				ti--
			}
		} else {
			rn, sz := utf8.DecodeLastRuneInString(routeString[0 : ti+1])
			if !unicode.IsSpace(rn) || (ti-sz > 0 && routeString[ti-sz] == '\\') {
				leftmostNonwhitespaceI = currentTagI - sz + 1
				if !foundRightmostNonwhitespace {
					rightmostNonwhitespaceI = currentTagI + 1
					foundRightmostNonwhitespace = true
				}
			}

			copy(currentTag[currentTagI-sz+1:currentTagI+1], routeString[ti-sz+1:ti+1])
			currentTagI -= sz

			if ti-sz > 0 && routeString[ti-sz] == '\\' {
				ti--
			}

			ti -= sz
		}
	}

	if ti < 0 {
		// False alarm - the closing ']' made it look as if there were tags, but
		// there weren't.
		return map[string]struct{}{}, len(routeString)
	}

	for {
		rn, sz := utf8.DecodeLastRuneInString(routeString[0:ti])
		if !unicode.IsSpace(rn) {
			break
		}
		ti -= sz
	}

	return tags, ti
}

func isDot(input string) bool {
	for i, c := range input {
		if unicode.IsSpace(c) {
			continue
		}
		if c == '.' {
			for _, cc := range input[i+1:] {
				if !unicode.IsSpace(cc) {
					return false
				}
			}
			return true
		}
		return false
	}

	return false
}

func isBlank(s string) bool {
	for _, c := range s {
		if !unicode.IsSpace(c) {
			return false
		}
	}
	return true
}

func containsNonLowerCase(s string) int {
	i := 0
	for {
		rn, sz := utf8.DecodeRuneInString(s[i:])
		if sz == 0 {
			return -1
		}
		if unicode.ToLower(rn) != rn {
			return i
		}
		i += sz
	}
}

func flatten[T any](a [][]T) []T {
	tot := 0
	for _, xs := range a {
		tot += len(xs)
	}
	flat := make([]T, tot)
	i := 0
	for _, xs := range a {
		for _, x := range xs {
			flat[i] = x
			i++
		}
	}
	return flat
}

func debugPrintParsedRoute(route []routeElement) string {
	var sb strings.Builder

	for i, elem := range route {
		if i != 0 {
			sb.WriteRune(' ')
		}

		switch elem.kind {
		case slash:
			sb.WriteRune('/')
		case constant:
			sb.WriteRune('\'')
			sb.WriteString(elem.value)
			sb.WriteRune('\'')
		case parameter:
			sb.WriteString("${")
			sb.WriteString(elem.value)
			sb.WriteRune('}')
		case integerParameter:
			sb.WriteString("$#{")
			sb.WriteString(elem.value)
			sb.WriteRune('}')
		case restParameter:
			sb.WriteString("$${")
			sb.WriteString(elem.value)
			sb.WriteRune('}')
		case singleGlob:
			sb.WriteRune('*')
		case doubleGlob:
			sb.WriteString("**")
		case noTrailingSlash:
			sb.WriteString("!/")
		}
	}

	return sb.String()
}
