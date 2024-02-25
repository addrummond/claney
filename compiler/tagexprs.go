package compiler

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/addrummond/claney/glob"
)

type tagExprKind int

const (
	tagExprNot tagExprKind = iota
	tagExprAnd
	tagExprOr
	tagExprLiteralTag
	tagExprLiteralMethod
	tagExprGlobTag
	tagExprGlobMethod
)

type TagExpr struct {
	kind     tagExprKind
	val      string
	children [2]*TagExpr
}

func EvalTagExpr(expr *TagExpr, tags map[string]struct{}, methods map[string]struct{}) bool {
	if expr == nil {
		return true
	}
	switch expr.kind {
	case tagExprLiteralTag:
		_, ok := tags[expr.val]
		return ok
	case tagExprLiteralMethod:
		_, ok := methods[expr.val]
		return ok
	case tagExprGlobTag:
		for m := range tags {
			if glob.Glob(expr.val, m) {
				return true
			}
		}
		return false
	case tagExprGlobMethod:
		for m := range methods {
			if glob.Glob(expr.val, m) {
				return true
			}
		}
		return false
	case tagExprAnd:
		return EvalTagExpr(expr.children[0], tags, methods) && EvalTagExpr(expr.children[1], tags, methods)
	case tagExprOr:
		return EvalTagExpr(expr.children[0], tags, methods) || EvalTagExpr(expr.children[1], tags, methods)
	case tagExprNot:
		return !EvalTagExpr(expr.children[0], tags, methods)
	}
	panic("Internal error in 'evalTagExpr': unknown tag expr kind")
}

func ParseTagExpr(input string) (expr *TagExpr, err error) {
	var tagErr *tagExprErr
	var rest string
	expr, rest, tagErr = parseTagExprHelper(input)
	if tagErr != nil {
		offset := len(input) - len(rest)
		line, col := getLineCol(input, offset)
		err = fmt.Errorf("%v:%v: %v", line, col, tagErr.message)
		return
	}
	if rest != "" {
		line, col := getLineCol(input, len(input))
		err = fmt.Errorf("%v:%v: Trailing input: '%v'", line, col, rest)
	}
	return
}

type tagExprErr struct {
	rest    string
	message string
}

func parseTagExprHelper(input string) (expr *TagExpr, rest string, err *tagExprErr) {
	rest = skipSpace(input)

	var tn *TagExpr
	tn, rest, err = getSimpleTagExpr(rest)
	if err != nil {
		return
	}
	if tn == nil {
		return
	}
	expr = tn

	for {
		rest = skipSpace(rest)
		r, sz := utf8.DecodeRuneInString(rest)
		if sz == 0 || r == ')' {
			break
		}

		rest = skipSpace(rest[sz:])

		if r == ')' {
			err = &tagExprErr{rest, "Misplaced ')'"}
			return
		}

		if r == '&' {
			var ntn *TagExpr
			ntn, rest, err = getSimpleTagExpr(rest)
			if err != nil {
				return
			}
			if ntn == nil {
				err = &tagExprErr{rest, "Trailing '&'"}
				return
			}
			expr = wrapLeft(tagExprAnd, expr, ntn)
		} else if r == '|' {
			var ntn *TagExpr
			ntn, rest, err = getSimpleTagExpr(rest)
			if err != nil {
				return
			}
			if ntn == nil {
				err = &tagExprErr{rest, "Trailing '|'"}
				return
			}
			expr = wrapLeft(tagExprOr, expr, ntn)
		} else {
			err = &tagExprErr{rest, fmt.Sprintf("Unexpected character '%c'", r)}
			return
		}
	}

	return
}

func getSimpleTagExpr(input string) (expr *TagExpr, rest string, err *tagExprErr) {
	var nbangs int
	nbangs, rest = getNBangs(input)
	if nbangs%2 == 1 {
		expr, rest, err = getSimpleTagExprHelper(rest)
		if err != nil {
			return
		}
		if expr == nil {
			err = &tagExprErr{rest, "Trailing '!'"}
		}
		expr = &TagExpr{
			kind:     tagExprNot,
			val:      "",
			children: [2]*TagExpr{expr},
		}
		return
	}
	expr, rest, err = getSimpleTagExprHelper(rest)
	return
}

func getSimpleTagExprHelper(input string) (expr *TagExpr, rest string, err *tagExprErr) {
	var sb strings.Builder

	rest = input
	r, sz := utf8.DecodeRuneInString(rest)
	if r == ')' {
		return
	}
	if r == '(' {
		rest = skipSpace(rest[sz:])
		expr, rest, err = parseTagExprHelper(rest)
		if err != nil {
			return
		}
		rest = skipSpace(rest)
		r, sz := utf8.DecodeRuneInString(rest)
		if r != ')' {
			err = &tagExprErr{rest, "No closing ')' found"}
			return
		}
		rest = rest[sz:]
		return
	}

	var isMethod bool
	if r == '[' {
		isMethod = true
		rest = skipSpace(rest[sz:])
		r, sz = utf8.DecodeRuneInString(rest)
	}

	for {
		if sz == 0 || (!isMethod && (r == '&' || r == '|' || r == '!' || r == ')' || r == '(' || r == '[' || r == ']' || unicode.IsSpace(r))) || (isMethod && r == ']') {
			break
		}
		if r == '\\' {
			r, sz := utf8.DecodeRuneInString(rest[sz:])
			if sz == 0 {
				sb.WriteRune('\\')
			} else if r == '*' { // to allow easier escaping of glob chars with '\*' instead of '\\*'
				rest = rest[sz:]
				sb.WriteString("\\*")
			} else {
				rest = rest[sz:]
				sb.WriteRune(r)
			}
		} else {
			sb.WriteRune(r)
		}
		rest = rest[sz:]

		r, sz = utf8.DecodeRuneInString(rest)
	}

	if isMethod {
		rest = skipSpace(rest)
		r, sz := utf8.DecodeRuneInString(rest)
		if r != ']' {
			err = &tagExprErr{rest, "Missing closing ']' for method"}
			return
		}
		rest = rest[sz:]
	}

	if sb.Len() != 0 {
		if isMethod {
			expr = literalMethod(strings.ToUpper(strings.TrimSpace(sb.String())))
		} else {
			expr = literalString(sb.String())
		}
	}
	return
}

func getNBangs(input string) (n int, rest string) {
	rest = input
	for {
		rest = skipSpace(rest)
		r, sz := utf8.DecodeRuneInString(rest)
		if r == '!' {
			rest = rest[sz:]
			n++
		} else {
			break
		}
	}
	return
}

func skipSpace(input string) string {
	for {
		r, sz := utf8.DecodeRuneInString(input)
		if !unicode.IsSpace(r) {
			return input
		}
		input = input[sz:]
	}
}

func literalString(tagName string) *TagExpr {
	kind := tagExprLiteralTag
	if glob.IsNonLiteral(tagName) {
		kind = tagExprGlobTag
	}
	return &TagExpr{
		kind:     kind,
		val:      tagName,
		children: [2]*TagExpr{},
	}
}

func literalMethod(methodName string) *TagExpr {
	kind := tagExprLiteralMethod
	if glob.IsNonLiteral(methodName) {
		kind = tagExprGlobMethod
	}
	return &TagExpr{
		kind:     kind,
		val:      methodName,
		children: [2]*TagExpr{},
	}
}

func wrapLeft(op tagExprKind, leftExpr, rightExpr *TagExpr) *TagExpr {
	if leftExpr == nil {
		return rightExpr
	}

	return &TagExpr{
		kind:     op,
		val:      "",
		children: [2]*TagExpr{leftExpr, rightExpr},
	}
}

func getLineCol(input string, offset int) (line, col int) {
	line = 1
	for i := 0; i < offset; {
		r, sz := utf8.DecodeRuneInString(input[i:])
		if sz == 0 {
			return
		}
		i += sz
		if r == '\n' {
			line++
			col = 0
		} else {
			col++
		}
	}
	return
}
