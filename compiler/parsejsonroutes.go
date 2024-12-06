package compiler

import (
	"fmt"
	"io"
	"strings"

	j "github.com/addrummond/jsonstream"
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
	var complexPatternElementStartToken j.Token

	var parser j.Parser

	for t := range parser.TokenizeAllowingComments(inp) {
		if e := t.AsError(); e != nil {
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

		if t.Kind == j.Comment {
			continue
		}

		switch s {
		case jpsInitial:
			if t.Kind != j.ArrayStart {
				errors = appendRouteErr(errors, ExpectedJSONRoutesToBeArray, t.Line, t.Col)
				return
			}
			s = jpsSeekingEntry
		case jpsSeekingEntry:
			currentEntry = RouteFileEntry{}
			switch t.Kind {
			case j.ObjectStart:
				s = jpsInEntry
				currentEntry = RouteFileEntry{line: t.Line}
				currentEntry.indent = currentIndent
			case j.ArrayStart:
				currentIndent++
			case j.ArrayEnd:
				currentIndent--
			default:
				fmt.Printf("BADKIND %v\n", t.Kind)
				if t.Kind != j.ObjectStart {
					errors = appendRouteErr(errors, ExpectedJSONRouteFileEntryToBeObject, t.Line, t.Col)
					return
				}
			}
		case jpsInEntry:
			currentEntry.indent = currentIndent
			k := string(t.Key)
			switch t.Kind {
			case j.String:
				if k != "name" {
					errors = appendRouteErr(errors, UnexpectedKeyInJSONRouteFile, t.Line, t.Col)
					return
				}
				currentEntry.name = t.AsString()
			case j.True, j.False:
				if k != "terminal" {
					errors = appendRouteErr(errors, UnexpectedKeyInJSONRouteFile, t.Line, t.Col)
					return
				}
				currentEntry.terminal = t.Kind == j.True
			case j.ArrayStart:
				if k == "tags" {
					s = jpsInTags
				} else if k == "methods" {
					s = jpsInMethods
				} else if k == "pattern" {
					s = jpsInPattern
				} else {
					errors = appendRouteErr(errors, UnexpectedKeyInJSONRouteFile, t.Line, t.Col)
				}
			case j.ObjectEnd:
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
					currentEntry.methods = map[string]struct{}{"GET": {}} // TODO is this duplicating logic elsewhere?
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
			case j.String:
				currentEntry.tags[t.AsString()] = struct{}{}
			case j.ArrayEnd:
				s = jpsInEntry
			default:
				errors = appendRouteErr(errors, UnexpectedTokenInJSONRouteFile, t.Line, t.Col)
				return
			}
		case jpsInMethods:
			switch t.Kind {
			case j.String:
				currentEntry.methods[string(t.Value)] = struct{}{}
			case j.ArrayEnd:
				s = jpsInEntry
			default:
				errors = appendRouteErr(errors, UnexpectedTokenInJSONRouteFile, t.Line, t.Col)
				return
			}
		case jpsInPattern:
			switch t.Kind {
			case j.String:
				val := t.AsString()
				if val == "/" {
					currentEntry.pattern = append(currentEntry.pattern, routeElement{slash, "", t.Line, t.Col})
				} else if val == "!/" {
					currentEntry.pattern = append(currentEntry.pattern, routeElement{noTrailingSlash, "", t.Line, t.Col})
				} else if strings.ContainsRune(val, '/') {
					errors = appendRouteErr(errors, NoSlashInsideJSONRoutePatternElement, t.Line, t.Col)
					return
				} else {
					if casePolicy == DisallowUpperCase {
						if lci := containsNonLowerCase(val); lci != -1 {
							errors = append(errors, routeError(UpperCaseCharInRoute, t.Line, t.Col+lci))
						}
					}
					currentEntry.pattern = append(currentEntry.pattern, routeElement{constant, val, t.Line, t.Col})
				}
			case j.ArrayStart:
				complexPatternElementStartToken = t
				s = jpsInPatternArrayElement
			case j.ArrayEnd:
				validationErrorKinds := validateRouteElems(0, currentIndent, currentEntry.pattern)
				if len(validationErrorKinds) > 0 {
					for _, k := range validationErrorKinds {
						errors = appendRouteErr(errors, k, t.Line, t.Col)
					}
				}
				s = jpsInEntry
			default:
				errors = appendRouteErr(errors, UnexpectedTokenInJSONRouteFile, t.Line, t.Col)
			}
		case jpsInPatternArrayElement:
			if t.Kind != j.String {
				errors = appendRouteErr(errors, FirstMemberOfJSONRouteFilePatternElementMustBeString, t.Line, t.Col)
				return
			}
			sval := t.AsString()
			switch sval {
			case "*":
				s = jpsInPatternArrayElementNoArg
				currentEntry.pattern = append(currentEntry.pattern, routeElement{singleGlob, "", complexPatternElementStartToken.Line, complexPatternElementStartToken.Col})
			case "**":
				s = jpsInPatternArrayElementNoArg
				currentEntry.pattern = append(currentEntry.pattern, routeElement{doubleGlob, "", complexPatternElementStartToken.Line, complexPatternElementStartToken.Col})
			case ":":
				s = jpsInPatternArrayElementParam
				currentEntry.pattern = append(currentEntry.pattern, routeElement{parameter, "", complexPatternElementStartToken.Line, complexPatternElementStartToken.Col})
			case ":**":
				s = jpsInPatternArrayElementParam
				currentEntry.pattern = append(currentEntry.pattern, routeElement{restParameter, "", complexPatternElementStartToken.Line, complexPatternElementStartToken.Col})
			default:
				errors = appendRouteErr(errors, BadFirstMemberOfJSONRouteFilePatternElement, t.Line, t.Col)
				return
			}
		case jpsInPatternArrayElementNoArg:
			if t.Kind != j.ArrayEnd {
				errors = appendRouteErr(errors, UnexpectedJSONRouteFilePatternElementMember, t.Line, t.Col)
				return
			}
			s = jpsInPattern
		case jpsInPatternArrayElementParam:
			if t.Kind != j.String {
				errors = appendRouteErr(errors, JSONRouteFilePatternElementParameterNameMustBeString, t.Line, t.Col)
				return
			}
			currentEntry.pattern[len(currentEntry.pattern)-1].value = t.AsString()
			s = jpsInPatternArrayElementNoArg
		}
	}

	return
}
