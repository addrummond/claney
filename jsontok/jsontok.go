// Package jsontok provides a simple JSON tokenizer that reports line and column
// information for tokens. It also supports /* */ and // comment syntax as an
// extension to standard JSON. You'd think these features would be easy to find
// in an existing library – but no!
package jsontok

import (
	"fmt"
	"iter"
	"unicode"
	"unicode/utf8"
)

type Kind int

const (
	ObjectStart      Kind = iota
	ObjectEnd             = iota
	ArrayStart            = iota
	ArrayEnd              = iota
	String                = iota | primval
	Number                = iota | primval
	True                  = iota | primval
	False                 = iota | primval
	Null                  = iota | primval
	Error                 = iota
	Comment               = iota
	NewDocumentStart      = iota
	colon
	comma
)

const primval = (1 << 30)

func (k Kind) String() string {
	switch k {
	case ObjectStart:
		return "ObjectStart"
	case ObjectEnd:
		return "ObjectEnd"
	case ArrayStart:
		return "ArrayStart"
	case ArrayEnd:
		return "ArrayEnd"
	case String:
		return "String"
	case Number:
		return "Number"
	case True:
		return "True"
	case False:
		return "False"
	case Null:
		return "Null"
	case Error:
		return "Error"
	case Comment:
		return "Comment"
	case colon:
		return "colon"
	case comma:
		return "comma"
	}
	return "<unknown Kind>"
}

type Token struct {
	Line     int
	Col      int
	Start    int
	End      int
	Key      []byte
	Kind     Kind
	Value    []byte
	ErrorMsg string
}

func (t Token) String() string {
	if t.Kind == Error {
		return fmt.Sprintf("%v:%v Error: %v\n", t.Line, t.Col, t.ErrorMsg)
	}
	var key string
	if len(t.Key) > 0 {
		key = string(t.Key) + "="
	}
	return fmt.Sprintf("%v:%v %v %v%s", t.Line, t.Col, t.Kind, key, t.Value)
}

func mkErr(line, col int, msg string) Token {
	return Token{
		Kind:     Error,
		Line:     line,
		Col:      col,
		ErrorMsg: msg,
	}
}

func Tokenize(inp []byte) iter.Seq[Token] {
	next_, stop := iter.Pull(rawTokenize(inp))

	// Wrap 'next_' function to skip (and yield) comments
	next := func(yield func(Token) bool) (t Token, ok bool, contIter bool) {
		contIter = true
		for {
			t, ok = next_()
			if !ok {
				return
			}
			if t.Kind != Comment {
				return
			}
			if !yield(t) {
				contIter = false
				return
			}
		}
	}

	var main func(yield func(Token) bool)
	var tokArray func(yield func(Token) bool) bool
	var tokObject func(yield func(Token) bool) bool

	main = func(yield func(Token) bool) {
		for i := 0; ; i++ {
			t, ok, contIter := next(yield)
			if !ok || !contIter {
				return
			}

			if i > 0 {
				yield(mkErr(t.Line, t.Col, "Trailing input"))
				return
			}

			switch t.Kind {
			case ObjectStart:
				if !yield(t) {
					return
				}
				if !tokObject(yield) {
					return
				}
			case ArrayStart:
				if !yield(t) {
					return
				}
				if !tokArray(yield) {
					return
				}
			case ObjectEnd, ArrayEnd, comma, colon:
				yield(mkErr(t.Line, t.Col, "Unexpected token"))
				return
			default:
				yield(t)
			}
		}
	}

	tokArray = func(yield func(Token) bool) bool {
		afterComma := false
		for {
			valtok, ok, contIter := next(yield)
			if !contIter {
				return false
			}
			if !ok {
				yield(mkErr(valtok.Line, valtok.Col, "Unexpected EOF (expected closing ']')"))
				return false
			}

			if valtok.Kind == ArrayEnd {
				if afterComma {
					yield(mkErr(valtok.Line, valtok.Col, "Trailing ','"))
					return false
				}
				yield(valtok)
				return true
			}

			switch valtok.Kind {
			case ArrayStart:
				if !yield(valtok) {
					return false
				}
				if !tokArray(yield) {
					return false
				}
			case ObjectStart:
				if !yield(valtok) {
					return false
				}
				if !tokObject(yield) {
					return false
				}
			case String, Number, True, False, Null:
				if !yield(valtok) {
					return false
				}
			default:
				yield(mkErr(valtok.Line, valtok.Col, "Unexpected token inside array"))
				return false
			}

			t, ok, contIter := next(yield)
			if !contIter {
				return false
			}
			if !ok {
				yield(mkErr(t.Line, t.Col, "Unexpected EOF inside array"))
				return false
			}

			if t.Kind == ArrayEnd {
				yield(t)
				return true
			}
			if t.Kind != comma {
				yield(mkErr(t.Line, t.Col, "Unexpected token inside array (expecing ',')"))
				return false
			}
			afterComma = true
		}
	}

	tokObject = func(yield func(Token) bool) bool {
		afterComma := false
		for {
			keytok, ok, contIter := next(yield)
			if !contIter {
				return false
			}
			if !ok {
				yield(mkErr(keytok.Line, keytok.Col, "Unexpected EOF (expected closing '}')"))
				return false
			}

			if keytok.Kind == ObjectEnd {
				if afterComma {
					yield(mkErr(keytok.Line, keytok.Col, "Trailing ','"))
					return false
				}
				yield(keytok)
				return true
			}

			if keytok.Kind != String {
				yield(mkErr(keytok.Line, keytok.Col, "Unexpected token inside object (expecting key)"))
				return false
			}

			t, ok, contIter := next(yield)
			if !contIter {
				return false
			}
			if !ok || t.Kind != colon {
				yield(mkErr(t.Line, t.Col, "Unexpected token inside object (expecting ':')"))
				return false
			}

			valtok, ok, contIter := next(yield)
			if !contIter {
				return false
			}
			if !ok {
				yield(mkErr(t.Line, t.Col, "Unexpected EOF"))
				return false
			}

			valtok.Key = keytok.Value

			switch valtok.Kind {
			case ArrayStart:
				if !yield(valtok) {
					return false
				}
				if !tokArray(yield) {
					return false
				}
			case ObjectStart:
				if !yield(valtok) {
					return false
				}
				if !tokObject(yield) {
					return false
				}
			case String, Number, True, False, Null:
				if !yield(valtok) {
					return false
				}
			default:
				yield(mkErr(t.Line, t.Col, "Unexpected token inside object"))
				return false
			}

			t, ok, contIter = next(yield)
			if !contIter {
				return false
			}
			if !ok {
				yield(mkErr(t.Line, t.Col, "Unexpected EOF"))
				return false
			}

			if t.Kind == ObjectEnd {
				yield(t)
				return true
			}
			if t.Kind != comma {
				yield(mkErr(t.Line, t.Col, "Unexpected token"))
				return false
			}
			afterComma = true
		}
	}

	return func(yield func(Token) bool) {
		defer stop()
		main(yield)
	}
}

func rawTokenize(inp []byte) iter.Seq[Token] {
	return func(yield func(Token) bool) {
		pos := 0
		lineStart := 0
		line := 1
		nextMustBeSep := false

	parseloop:
		for {
			if pos >= len(inp) {
				return
			}

			c := inp[pos]
			if nextMustBeSep {
				switch c {
				case ' ', '\r', '\n', '\t', '/', ':', ',', '[', ']', '{', '}':
					nextMustBeSep = false
				default:
					yield(mkErr(line, pos-lineStart, "Unexpected character"))
					return
				}
			}

			switch c {
			case ' ', '\r', '\n', '\t':
				pos++
				if c == '\n' {
					line++
					lineStart = pos - 1
				}
				continue parseloop
			case '/':
				start := pos
				startLine := line
				startCol := pos - lineStart
				pos++
				if pos >= len(inp) {
					yield(mkErr(line, pos-lineStart, "Unexpected '/'"))
					return
				}
				switch inp[pos] {
				case '*':
					for {
						pos++
						if pos >= len(inp) {
							yield(mkErr(line, pos-lineStart, "Unexpected EOF inside comment"))
							return
						}
						if inp[pos] == '\n' {
							line++
							lineStart = pos
							pos++
						} else if inp[pos] == '*' {
							pos++
							if pos >= len(inp) {
								yield(mkErr(line, pos-lineStart, "Unexpected EOF inside /* ... */ comment"))
								return
							}
							if inp[pos] == '/' {
								if !yield(Token{Line: startLine, Col: startCol, Start: start, End: pos, Kind: Comment, Value: inp[start : pos+1]}) {
									return
								}
								pos++
								continue parseloop
							}
						}
					}
				case '/':
					for {
						pos++
						if pos >= len(inp) {
							yield(mkErr(line, pos-lineStart, "Unexpected EOF inside // comment"))
							return
						}
						if inp[pos] == '\n' {
							if !yield(Token{Line: startLine, Col: startCol, Start: start, End: pos - 1, Kind: Comment, Value: inp[start:pos]}) {
								return
							}
							pos++
							line++
							lineStart = pos - 1
							continue parseloop
						}
					}
				default:
					yield(mkErr(line, pos-lineStart, "Unexpected '/'"))
					return
				}
			case ':':
				if !yield(Token{Line: line, Col: pos - lineStart, Start: pos, End: pos, Kind: colon}) {
					return
				}
				pos++
			case ',':
				if !yield(Token{Line: line, Col: pos - lineStart, Start: pos, End: pos, Kind: comma}) {
					return
				}
				pos++
			case '[':
				if !yield(Token{Line: line, Col: pos - lineStart, Start: pos, End: pos, Kind: ArrayStart}) {
					return
				}
				pos++
			case '{':
				if !yield(Token{Line: line, Col: pos - lineStart, Start: pos, End: pos, Kind: ObjectStart}) {
					return
				}
				pos++
			case ']':
				if !yield(Token{Line: line, Col: pos - lineStart, Start: pos, End: pos, Kind: ArrayEnd}) {
					return
				}
				pos++
			case '}':
				if !yield(Token{Line: line, Col: pos - lineStart, Start: pos, End: pos, Kind: ObjectEnd}) {
					return
				}
				pos++
			case 't':
				start := pos
				startCol := pos - lineStart
				if pos+3 >= len(inp) || inp[pos+1] != 'r' || inp[pos+2] != 'u' || inp[pos+3] != 'e' {
					yield(mkErr(line, pos-lineStart, "Unexpected 't'"))
					return
				}
				pos += 4
				nextMustBeSep = true
				if !yield(Token{Line: line, Col: startCol, Start: start, End: pos, Kind: True}) {
					return
				}
			case 'f':
				start := pos
				startCol := pos - lineStart
				if pos+4 >= len(inp) || inp[pos+1] != 'a' || inp[pos+2] != 'l' || inp[pos+3] != 's' || inp[pos+4] != 'e' {
					yield(mkErr(line, startCol, "Unexpected 'f'"))
					return
				}
				pos += 5
				nextMustBeSep = true
				if !yield(Token{Line: line, Col: startCol, Start: start, End: pos, Kind: False}) {
					return
				}
			case 'n':
				start := pos
				startCol := pos - lineStart
				if pos+3 >= len(inp) || inp[pos+1] != 'u' || inp[pos+2] != 'l' || inp[pos+3] != 'l' {
					yield(mkErr(line, startCol, "Unexpected 'n'"))
					return
				}
				pos += 4
				nextMustBeSep = true
				if !yield(Token{Line: line, Col: startCol, Start: start, End: pos, Kind: Null}) {
					return
				}
			case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
				start := pos
				startCol := pos - lineStart
				if c == '-' {
					pos++
					if pos >= len(inp) {
						yield(mkErr(line, pos-lineStart, "Unexpected EOF"))
						return
					}
					if inp[pos] < '0' || inp[pos] > '9' {
						yield(mkErr(line, pos-lineStart, "Unexpected char after '-'"))
						return
					}
				}
				// Peek to check that we don't have leading zero
				if inp[pos] == '0' && pos+1 < len(inp) && inp[pos+1] >= '0' && inp[pos+1] <= '9' {
					yield(mkErr(line, pos-lineStart, "Leading zeros not permitted"))
					return
				}
				pos++
				for pos < len(inp) && inp[pos] >= '0' && inp[pos] <= '9' {
					pos++
				}
				if pos < len(inp) && inp[pos] == '.' {
					pos++
					if pos >= len(inp) || inp[pos] < '0' || inp[pos] > '9' {
						yield(mkErr(line, pos-lineStart, "Expected digit after '.' in number"))
						return
					}
					for {
						pos++
						if pos >= len(inp) || inp[pos] < '0' || inp[pos] > '9' {
							break
						}
					}
				}
				if pos < len(inp) && (inp[pos] == 'e' || inp[pos] == 'E') {
					pos++
					if pos >= len(inp) {
						yield(mkErr(line, pos-lineStart, "Unexpected EOF"))
						return
					}
					if inp[pos] == '+' || inp[pos] == '-' {
						pos++
					}
					if pos >= len(inp) {
						yield(mkErr(line, pos-lineStart, "Unexpected EOF"))
						return
					}
					if inp[pos] < '0' || inp[pos] > '9' {
						yield(mkErr(line, pos-lineStart, "Unexpected digit following 'e' in number"))
						return
					}
					pos++
					for pos < len(inp) && inp[pos] >= '0' && inp[pos] <= '9' {
						pos++
					}
				}
				nextMustBeSep = true
				if !yield(Token{Line: line, Col: startCol, Start: start, End: pos - 1, Kind: Number, Value: inp[start:pos]}) {
					return
				}
			case '"':
				start := pos
				startCol := pos - lineStart
				pos++
				var val []byte
				canUseInpSlice := true
				for {
					if pos >= len(inp) {
						yield(mkErr(line, pos-lineStart, "Unexpected EOF in string"))
						return
					}
					switch inp[pos] {
					case '"':
						if canUseInpSlice {
							canUseInpSlice = false
							val = inp[start+1 : pos]
						}
						if !yield(Token{Line: line, Col: startCol, Start: start, End: pos, Kind: String, Value: val}) {
							return
						}
						pos++
						continue parseloop
					case '\\':
						if canUseInpSlice {
							canUseInpSlice = false
							val = append(val, inp[start+1:pos]...)
						}
						pos++
						if pos >= len(inp) {
							yield(mkErr(line, pos-lineStart, "Unexpected EOF in string"))
							return
						}
						switch inp[pos] {
						case '"', '\\', '/':
							val = append(val, inp[pos])
							pos++
						case 'b':
							val = append(val, '\b')
							pos++
						case 'f':
							val = append(val, '\f')
							pos++
						case 'n':
							val = append(val, '\n')
							pos++
						case 'r':
							val = append(val, '\r')
							pos++
						case 't':
							val = append(val, '\t')
							pos++
						case 'u':
							if pos+4 >= len(inp) {
								yield(mkErr(line, pos-lineStart, "Unexpected EOF"))
								return
							}
							d1 := hexVal(inp[pos+1])
							d2 := hexVal(inp[pos+2])
							d3 := hexVal(inp[pos+3])
							d4 := hexVal(inp[pos+4])
							if d1 == -1 || d2 == -1 || d3 == -1 || d4 == -1 {
								yield(mkErr(line, pos-lineStart, "Bad '\\uXXXX' escape in string"))
								return
							}
							runeVal := d1*16*16*16 + d2*16*16 + d3*16 + d4
							val = utf8.AppendRune(val, rune(runeVal))
							pos += 5
						default:
							yield(mkErr(line, pos-lineStart, "Unexpected character after '\\' in string"))
							return
						}
					default:
						r, sz := utf8.DecodeRune(inp[pos:])
						// DEL is permitted according to
						// https://datatracker.ietf.org/doc/html/rfc7159
						if unicode.IsControl(r) && r != 0x7F {
							yield(mkErr(line, pos-lineStart, "Illegal control char inside string"))
							return
						}
						if r == utf8.RuneError {
							if sz == 0 {
								yield(mkErr(line, pos-lineStart, "Unexpected EOF inside string"))
								return
							} else {
								yield(mkErr(line, pos-lineStart, "UTF-8 decoding error inside string"))
								return
							}
						}
						if !canUseInpSlice {
							val = append(val, inp[pos:pos+sz]...)
						}
						pos += sz
					}
				}
			default:
				r, _ := utf8.DecodeRune(inp[pos:])
				yield(mkErr(line, pos-lineStart, fmt.Sprintf("Unexpected char '%v'", r)))
				return
			}
		}
	}
}

func hexVal(d byte) int {
	if d >= '0' && d <= '9' {
		return int(d) - '0'
	}
	if d >= 'a' && d <= 'f' {
		return int(d) - 'a' + 10
	}
	if d >= 'A' && d <= 'F' {
		return int(d) - 'A' + 10
	}
	return -1
}
