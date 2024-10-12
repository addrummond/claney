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

func appendRouteErr(errors []RouteError, kind RouteErrorKind, line, col int) []RouteError {
	return append(errors, RouteError{
		Kind: kind,
		Line: line,
		Col:  col,
	})
}

func ParseJsonRouteFile(input io.Reader, casePolicy CasePolicy) (entries []RouteFileEntry, errors []RouteError) {
	inp, err := io.ReadAll(input)
	if err != nil {
		errors = appendRouteErr(errors, IOError, -1, -1)
		return
	}

	s := jpsInitial
	currentEntry := RouteFileEntry{}
	currentIndent := 0

	for t := range jsontok.Tokenize(inp) {
		if t.Kind == jsontok.Error {
			errors = append(errors,
				RouteError{
					Kind:      InvalidJsonInJSONRouteFile,
					Line:      t.Line,
					Col:       t.Col,
					JsonError: t,
				},
			)
			return
		}

		switch s {
		case jpsInitial:
			if t.Kind != jsontok.ArrayStart {
				errors = appendRouteErr(errors, ExpectedJSONRoutesToBeArray, t.Line, t.Col)
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
				fmt.Printf("BADKIND %v\n", t.Kind)
				if t.Kind != jsontok.ObjectStart {
					errors = appendRouteErr(errors, ExpectedJSONRouteFileEntryToBeObject, t.Line, t.Col)
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
					errors = appendRouteErr(errors, UnexpectedKeyInJSONRouteFile, t.Line, t.Col)
					return
				}
				currentEntry.name = string(t.Value)
			case jsontok.True, jsontok.False:
				if k != "terminal" {
					errors = appendRouteErr(errors, UnexpectedKeyInJSONRouteFile, t.Line, t.Col)
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
					errors = appendRouteErr(errors, UnexpectedKeyInJSONRouteFile, t.Line, t.Col)
				}
			case jsontok.ObjectEnd:
				s = jpsSeekingEntry
				if currentEntry.name == "" {
					errors = appendRouteErr(errors, JSONRouteMissingNameField, t.Line, t.Col)
					return
				}
				if currentEntry.pattern == nil {
					errors = appendRouteErr(errors, JSONRouteMissingPatternField, t.Line, t.Col)
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
				errors = appendRouteErr(errors, UnexpectedTokenInJSONRouteFile, t.Line, t.Col)
				return
			}
		case jpsInTags:
			switch t.Kind {
			case jsontok.String:
				currentEntry.tags[string(t.Value)] = struct{}{}
			case jsontok.ArrayEnd:
				s = jpsInEntry
			default:
				errors = appendRouteErr(errors, UnexpectedTokenInJSONRouteFile, t.Line, t.Col)
				return
			}
		case jpsInMethods:
			switch t.Kind {
			case jsontok.String:
				currentEntry.methods[string(t.Value)] = struct{}{}
			case jsontok.ArrayEnd:
				s = jpsInEntry
			default:
				errors = appendRouteErr(errors, UnexpectedTokenInJSONRouteFile, t.Line, t.Col)
				return
			}
		case jpsInPattern:
			switch t.Kind {
			case jsontok.String:
				val := string(t.Value)
				currentEntry.pattern = append(currentEntry.pattern, routeElement{slash, "", t.Col})
				if strings.ContainsRune(val, '/') {
					errors = appendRouteErr(errors, NoSlashInsideJSONRoutePatternElement, t.Line, t.Col)
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
						errors = appendRouteErr(errors, k, t.Line, t.Col)
					}
				}
				s = jpsInEntry
			default:
				errors = appendRouteErr(errors, UnexpectedTokenInJSONRouteFile, t.Line, t.Col)
			}
		case jpsInPatternArrayElement:
			if t.Kind != jsontok.String {
				errors = appendRouteErr(errors, FirstMemberOfPatternElementMustBeString, t.Line, t.Col)
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
				errors = appendRouteErr(errors, BadFirstMemberOfPatternElement, t.Line, t.Col)
				return
			}
		case jpsInPatternArrayElementNoArg:
			if t.Kind != jsontok.ArrayEnd {
				errors = appendRouteErr(errors, UnexpectedPatternElementMember, t.Line, t.Col)
				return
			}
			s = jpsInPattern
		case jpsInPatternArrayElementParam:
			if t.Kind != jsontok.String {
				errors = appendRouteErr(errors, ParameterNameMustBeString, t.Line, t.Col)
				return
			}
			currentEntry.pattern[len(currentEntry.pattern)-1].value = string(t.Value)
			s = jpsInPatternArrayElementNoArg
		}
	}

	return
}
