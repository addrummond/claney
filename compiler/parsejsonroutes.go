package compiler

import (
	"fmt"
	"io"
	"strings"

	"github.com/addrummond/claney/jsontok"
)

type jsonParseState int

const (
	jpsInitial jsonParseState = iota
	jpsSeekingEntry
	jpsInEntry
	jpsInTags
	jpsInMethods
	jpsInPattern
	jpsInPatternArrayElement
	jpsInPatternArrayElementNoArg
	jpsInPatternArrayElementParam
)

func appendRouteErr(errors []RouteError, kind RouteErrorKind, line, col int, message string) []RouteError {
	return append(errors, RouteError{kind, line, col, message, 0, nil, nil, nil})
}

func ParseJsonRouteFile(input io.Reader, casePolicy CasePolicy) (entries []RouteFileEntry, errors []RouteError) {
	inp, err := io.ReadAll(input)
	if err != nil {
		errors = appendRouteErr(errors, IOError, -1, -1, "")
		return
	}

	s := jpsInitial
	currentEntry := RouteFileEntry{}
	currentIndent := 0

	for t := range jsontok.Tokenize(inp) {
		fmt.Printf("%v:  %+v\n", s, t)

		switch s {
		case jpsInitial:
			if t.Kind != jsontok.ArrayStart {
				errors = appendRouteErr(errors, ExpectedJSONRoutesToBeArray, t.Line, t.Col, "Expected JSON routes file to be array")
				return
			}
			s = jpsSeekingEntry
		case jpsSeekingEntry:
			currentEntry = RouteFileEntry{}
			switch t.Kind {
			case jsontok.ObjectStart:
				s = jpsInEntry
				currentEntry = RouteFileEntry{}
				currentEntry.indent = currentIndent
			case jsontok.ArrayStart:
				currentIndent++
			case jsontok.ArrayEnd:
				currentIndent--
			default:
				if t.Kind != jsontok.ObjectStart {
					errors = appendRouteErr(errors, ExpectedJSONRouteFileEntryToBeObject, t.Line, t.Col, "Expected JSON route file entry to be an object")
					return
				}
			}
		case jpsInEntry:
			currentEntry.indent = currentIndent
			currentEntry.line = t.Line
			k := string(t.Key)
			switch t.Kind {
			case jsontok.String:
				if k != "name" {
					errors = appendRouteErr(errors, UnexpectedKeyInJSONRouteFile, t.Line, t.Col, "Unexpected object key or bad value for key")
					return
				}
				currentEntry.name = string(t.Value)
			case jsontok.True, jsontok.False:
				if k != "terminal" {
					errors = appendRouteErr(errors, UnexpectedKeyInJSONRouteFile, t.Line, t.Col, "Unexpected object key or bad value for key")
					return
				}
				currentEntry.terminal = string(t.Value) == "true"
			case jsontok.ArrayStart:
				if k == "tags" {
					s = jpsInTags
				} else if k == "methods" {
					s = jpsInMethods
				} else if k == "pattern" {
					s = jpsInPattern
				} else {
					errors = appendRouteErr(errors, UnexpectedKeyInJSONRouteFile, t.Line, t.Col, "Unexpected object key or bad value for key")
				}
			case jsontok.ObjectEnd:
				s = jpsSeekingEntry
				if currentEntry.name == "" {
					errors = appendRouteErr(errors, JSONRouteMissingNameField, t.Line, t.Col, "Route is missing 'name' field")
					return
				}
				if currentEntry.pattern == nil {
					errors = appendRouteErr(errors, JSONRouteMissingPatternField, t.Line, t.Col, "Route is missing 'pattern' field")
					return
				}
				if currentEntry.tags == nil {
					currentEntry.tags = make(map[string]struct{})
					currentEntry.tags["GET"] = struct{}{} // TODO is this duplicating logic elsewhere?
				}
				if currentEntry.methods == nil {
					currentEntry.methods = make(map[string]struct{})
				}
				entries = append(entries, currentEntry)
			default:
				errors = appendRouteErr(errors, UnexpectedTokenInJSONRouteFile, t.Line, t.Col, "Unexpected token")
				return
			}
		case jpsInTags:
			switch t.Kind {
			case jsontok.String:
				currentEntry.tags[string(t.Value)] = struct{}{}
			case jsontok.ArrayEnd:
				s = jpsInEntry
			default:
				errors = appendRouteErr(errors, UnexpectedTokenInJSONRouteFile, t.Line, t.Col, "Unexpected token")
				return
			}
		case jpsInMethods:
			switch t.Kind {
			case jsontok.String:
				currentEntry.methods[string(t.Value)] = struct{}{}
			case jsontok.ArrayEnd:
				s = jpsInEntry
			default:
				errors = appendRouteErr(errors, UnexpectedTokenInJSONRouteFile, t.Line, t.Col, "Unexpected token")
				return
			}
		case jpsInPattern:
			switch t.Kind {
			case jsontok.String:
				val := string(t.Value)
				currentEntry.pattern = append(currentEntry.pattern, routeElement{slash, "", t.Col})
				if strings.ContainsRune(val, '/') {
					errors = appendRouteErr(errors, NoSlashInsideJSONRoutePatternElement, t.Line, t.Col, "No '/' allowed inside route pattern element")
					return
				} else {
					currentEntry.pattern = append(currentEntry.pattern, routeElement{constant, val, t.Col})
				}
			case jsontok.ArrayStart:
				s = jpsInPatternArrayElement
			case jsontok.ArrayEnd:
				reKinds := validateRouteElems(0, currentIndent, currentEntry.pattern)
				if len(reKinds) > 0 {
					for _, k := range reKinds {
						errors = appendRouteErr(errors, k, t.Line, t.Col, "Bad route pattern")
					}
				}
				s = jpsInEntry
			default:
				errors = appendRouteErr(errors, UnexpectedTokenInJSONRouteFile, t.Line, t.Col, "Unexpected token")
				return
			}
		case jpsInPatternArrayElement:
			if t.Kind != jsontok.String {
				errors = appendRouteErr(errors, FirstMemberOfPatternElementMustBeString, t.Line, t.Col, "First member of array route element must be string")
				return
			}
			sval := string(t.Value)
			switch sval {
			case "*":
				s = jpsInPatternArrayElementNoArg
				currentEntry.pattern = append(currentEntry.pattern, routeElement{singleGlob, "", t.Col})
			case "**":
				s = jpsInPatternArrayElementNoArg
				currentEntry.pattern = append(currentEntry.pattern, routeElement{doubleGlob, "", t.Col})
			case "!/":
				// TODO check that no trailing slash being at end is validated somewhere
				s = jpsInPatternArrayElementNoArg
				currentEntry.pattern = append(currentEntry.pattern, routeElement{noTrailingSlash, "", t.Col})
			case ":":
				s = jpsInPatternArrayElementParam
				currentEntry.pattern = append(currentEntry.pattern, routeElement{parameter, "", t.Col})
			case ":**":
				s = jpsInPatternArrayElementParam
				currentEntry.pattern = append(currentEntry.pattern, routeElement{restParameter, "", t.Col})
			default:
				errors = appendRouteErr(errors, BadFirstMemberOfPatternElement, t.Line, t.Col, "Unrecognized first member of pattern element")
				return
			}
		case jpsInPatternArrayElementNoArg:
			if t.Kind != jsontok.ArrayEnd {
				errors = appendRouteErr(errors, UnexpectedPatternElementMember, t.Line, t.Col, "Unexpected additional member of pattern element")
				return
			}
			s = jpsInPattern
		case jpsInPatternArrayElementParam:
			if t.Kind != jsontok.String {
				errors = appendRouteErr(errors, ParameterNameMustBeString, t.Line, t.Col, "Parameter name must be string")
				return
			}
			currentEntry.pattern[len(currentEntry.pattern)-1].value = string(t.Value)
			s = jpsInPatternArrayElementNoArg
		}
	}

	return
}
