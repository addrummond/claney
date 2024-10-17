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
	var complexPatternElementStartToken jsontok.Token

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

		if t.Kind == jsontok.Comment {
			continue
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
				currentEntry = RouteFileEntry{line: t.Line}
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
				currentEntry.terminal = t.Kind == jsontok.True
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
				if val == "/" {
					currentEntry.pattern = append(currentEntry.pattern, routeElement{slash, "", t.Line, t.Col})
				} else if val == "!/" {
					currentEntry.pattern = append(currentEntry.pattern, routeElement{noTrailingSlash, "", t.Line, t.Col})
				} else if strings.ContainsRune(val, '/') {
					errors = appendRouteErr(errors, NoSlashInsideJSONRoutePatternElement, t.Line, t.Col)
					return
				} else {
					currentEntry.pattern = append(currentEntry.pattern, routeElement{constant, val, t.Line, t.Col})
				}
			case jsontok.ArrayStart:
				complexPatternElementStartToken = t
				s = jpsInPatternArrayElement
			case jsontok.ArrayEnd:
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
			if t.Kind != jsontok.String {
				errors = appendRouteErr(errors, FirstMemberOfJSONRouteFilePatternElementMustBeString, t.Line, t.Col)
				return
			}
			sval := string(t.Value)
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
			if t.Kind != jsontok.ArrayEnd {
				errors = appendRouteErr(errors, UnexpectedJSONRouteFilePatternElementMember, t.Line, t.Col)
				return
			}
			s = jpsInPattern
		case jpsInPatternArrayElementParam:
			if t.Kind != jsontok.String {
				errors = appendRouteErr(errors, JSONRouteFilePatternElementParameterNameMustBeString, t.Line, t.Col)
				return
			}
			currentEntry.pattern[len(currentEntry.pattern)-1].value = string(t.Value)
			s = jpsInPatternArrayElementNoArg
		}
	}

	return
}
