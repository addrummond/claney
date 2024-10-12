package jsontok

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestTokenize(t *testing.T) {
	const input = `
["xxx", ["ba\u0041r"], "yyy", [ /* a comment inside */ ] // a comment
, {"aaa": "bbb", "x": "y"}, "bbb", {"numeric": 1.4e-99 } ]
`

	const expectedTokSeq = `
{2:1 ArrayStart  "["}
{2:2 String xxx "\"xxx\""}
{2:9 ArrayStart  "["}
{2:10 String baAr "\"ba\\u0041r\""}
{2:21 ArrayEnd  "]"}
{2:24 String yyy "\"yyy\""}
{2:31 ArrayStart  "["}
{2:33 Comment /* a comment inside */ "/* a comment inside */"}
{2:56 ArrayEnd  "]"}
{2:58 Comment // a comment "// a comment"}
{3:3 ObjectStart  "{"}
{3:11 String aaa=bbb "\"bbb\""}
{3:23 String x=y "\"y\""}
{3:26 ObjectEnd  "}"}
{3:29 String bbb "\"bbb\""}
{3:36 ObjectStart  "{"}
{3:48 Number numeric=1.4e-99 "1.4e-99"}
{3:56 ObjectEnd  "}"}
{3:58 ArrayEnd  "]"}
`

	t.Logf("%v\n", tokSeq(input))

	if strings.TrimSpace(expectedTokSeq) != strings.TrimSpace(tokSeq(input)) {
		t.Fatalf("Unexpected token sequence")
	}
}

func TestBadCommasInArrays(t *testing.T) {
	t.Run("Empty list", func(t *testing.T) {
		const input = "[]"
		if !succeeds(input) {
			t.Errorf("Expected to succeed")
		}
	})
	t.Run("Commas A-ok", func(t *testing.T) {
		const input = "[1,2,3]"
		if !succeeds(input) {
			t.Errorf("Expected to succeed")
		}
	})
	t.Run("Commas A-ok 1 elem", func(t *testing.T) {
		const input = "[1]"
		if !succeeds(input) {
			t.Errorf("Expected to succeed")
		}
	})
	t.Run("Initial comma", func(t *testing.T) {
		const input = "[,1,2,3]"
		if succeeds(input) {
			t.Errorf("Expected to fail")
		}
	})
	t.Run("Trailing comma 3 elems", func(t *testing.T) {
		const input = "[1,2,3,]"
		if succeeds(input) {
			t.Errorf("Expected to fail")
		}
	})
	t.Run("Trailing comma 2 elems", func(t *testing.T) {
		const input = "[1,2,]"
		if succeeds(input) {
			t.Errorf("Expected to fail")
		}
	})
	t.Run("Trailing comma 1 elem", func(t *testing.T) {
		const input = "[1,]"
		if succeeds(input) {
			t.Errorf("Expected to fail")
		}
	})
	t.Run("Comma only", func(t *testing.T) {
		const input = "[,]"
		if succeeds(input) {
			t.Errorf("Expected to fail")
		}
	})
}

func TestNestedArrays(t *testing.T) {
	t.Run("Nested empty arrays", func(t *testing.T) {
		const input = `[[[[]]]]`
		if !succeeds(input) {
			t.Errorf("Expected to succeed")
		}
	})
	t.Run("Lots of nested empty arrays", func(t *testing.T) {
		const input = `[[[[[[[[[[[[[[]]]]]]]]]]]]]]`
		if !succeeds(input) {
			t.Errorf("Expected to succeed")
		}
	})
	t.Run("Nested array with empty array members", func(t *testing.T) {
		const input = `[[[[], []]]]`
		if !succeeds(input) {
			t.Errorf("Expected to succeed")
		}
	})
	t.Run("Nested array with various members", func(t *testing.T) {
		const input = `[[[[1], 2, [], 4]],9]`
		if !succeeds(input) {
			t.Errorf("Expected to succeed")
		}
	})
}

func TestNestedObjects(t *testing.T) {
	t.Run("Empty object", func(t *testing.T) {
		const input = `{ }`
		if !succeeds(input) {
			t.Errorf("Expected to succeed")
		}
	})
	t.Run("Nested objects arrays", func(t *testing.T) {
		const input = `{"f":{"g":{}, "x":{}}}`
		if !succeeds(input) {
			t.Errorf("Expected to succeed")
		}
	})
}

func TestEmptyArrays(t *testing.T) {
	t.Run("Simple case", func(t *testing.T) {
		const input = `[]`
		if !succeeds(input) {
			t.Errorf("Expected to succeed")
		}
	})
	t.Run("Nested 2", func(t *testing.T) {
		const input = `[[]]`
		if !succeeds(input) {
			t.Errorf("Expected to succeed")
		}
	})
	t.Run("Nested 2 with spaces", func(t *testing.T) {
		const input = `[ [ ] ]`
		if !succeeds(input) {
			t.Errorf("Expected to succeed")
		}
	})
	t.Run("Nested 3", func(t *testing.T) {
		const input = `[[[]]]`
		if !succeeds(input) {
			t.Errorf("Expected to succeed")
		}
	})
	t.Run("Nested 3 with spaces", func(t *testing.T) {
		const input = `[ [ [ ] ] ]`
		if !succeeds(input) {
			t.Errorf("Expected to succeed")
		}
	})
}

func TestNonStringKeysInObjects(t *testing.T) {
	t.Run("Simple case", func(t *testing.T) {
		const input = `{1:2}`
		if succeeds(input) {
			t.Errorf("Expected to fail")
		}
	})
}

func TestBadCommasInObjects(t *testing.T) {
	t.Run("Commas A-ok", func(t *testing.T) {
		const input = `{"foo":1,"bar":2,"amp":3}`
		if !succeeds(input) {
			t.Errorf("Expected to succeed")
		}
	})
	t.Run("Initial comma", func(t *testing.T) {
		const input = `{,"foo":1,"bar":2,"amp":3}`
		if succeeds(input) {
			t.Errorf("Expected to fail")
		}
	})
	t.Run("Trailing comma 3 elems", func(t *testing.T) {
		const input = `{"foo":1,"bar":2,"amp":3,}`
		if succeeds(input) {
			t.Errorf("Expected to fail")
		}
	})
	t.Run("Trailing comma 1 elem", func(t *testing.T) {
		const input = `{"foo":1,}`
		if succeeds(input) {
			t.Errorf("Expected to fail")
		}
	})
	t.Run("Comma only", func(t *testing.T) {
		const input = `{,}`
		if succeeds(input) {
			t.Errorf("Expected to fail")
		}
	})
	t.Run("Trailing comma in middle of only entry", func(t *testing.T) {
		const input = `{"foo":,}`
		if succeeds(input) {
			t.Errorf("Expected to fail")
		}
	})
	t.Run("Trailing comma in middle of entry", func(t *testing.T) {
		const input = `{"bar":"amp","foo":,}`
		if succeeds(input) {
			t.Errorf("Expected to fail")
		}
	})
}

func TestEmptyObject(t *testing.T) {
	t.Run("Empty object no spaces", func(t *testing.T) {
		const input = `{}`
		if !succeeds(input) {
			t.Errorf("Expected to succeed")
		}
	})
	t.Run("Empty object spaces", func(t *testing.T) {
		const input = `{    }`
		if !succeeds(input) {
			t.Errorf("Expected to succeed")
		}
	})
}

func TestNumericZeros(t *testing.T) {
	t.Run("0", func(t *testing.T) {
		const input = "0"
		if !succeeds(input) {
			t.Errorf("Expected to succeed")
		}
	})
	t.Run("-0", func(t *testing.T) {
		const input = "-0"
		if !succeeds(input) {
			t.Errorf("Expected to succeed")
		}
	})
	t.Run("-00", func(t *testing.T) {
		const input = "-00"
		if succeeds(input) {
			t.Errorf("Expected to fail")
		}
	})
	t.Run("00", func(t *testing.T) {
		const input = "00"
		if succeeds(input) {
			t.Errorf("Expected to fail")
		}
	})
	t.Run("01", func(t *testing.T) {
		const input = "01"
		if succeeds(input) {
			t.Errorf("Expected to fail")
		}
	})
	t.Run("-01", func(t *testing.T) {
		const input = "-01"
		if succeeds(input) {
			t.Errorf("Expected to fail")
		}
	})
}

func TestTrailingInput(t *testing.T) {
	t.Run("No trailing input", func(t *testing.T) {
		const input = "{}"
		if !succeeds(input) {
			t.Errorf("Expected to succeed")
		}
	})
	t.Run("Trailing whitespace", func(t *testing.T) {
		const input = "{} \n\t\n"
		if !succeeds(input) {
			t.Errorf("Expected to succeed")
		}
	})
	t.Run("Trailing non-whitespace", func(t *testing.T) {
		const input = "{}1"
		if succeeds(input) {
			t.Errorf("Expected to fail")
		}
	})
	t.Run("Trailing whitespace followed by non-whitespace", func(t *testing.T) {
		const input = "{} \n\t1"
		if succeeds(input) {
			t.Errorf("Expected to fail")
		}
	})
}

func TestJSONTestSuite(t *testing.T) {
	for filename, base64Contents := range jsonTestInputs {
		contents, err := base64.StdEncoding.DecodeString(base64Contents)
		if err != nil {
			t.Fatalf("Error decoding base64 input: %v", err)
		}

		if err != nil {
			t.Fatal(err)
		}
		succeeded := true
		for t := range Tokenize(contents) {
			if t.Kind == Error {
				succeeded = false
				break
			}
		}
		t.Logf("Parsing %v", filename)
		if strings.HasPrefix(filename, "y_") && !succeeded {
			t.Errorf("Expected %v to succeed", filename)
		} else if strings.HasPrefix(filename, "n_") && succeeded {
			t.Errorf("Expected %v to fail", filename)
		}
	}
}

func tokSeq(inp string) string {
	var sb strings.Builder
	i := 0
	for t := range Tokenize([]byte(inp)) {
		repr := inp[t.Start : t.End+1]
		jrepr, err := json.Marshal(repr)
		if err != nil {
			panic(err)
		}
		if i != 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(fmt.Sprintf("{%v %s}", t, jrepr))
		i++
	}
	return sb.String()
}

func succeeds(inp string) bool {
	for t := range Tokenize([]byte(inp)) {
		if t.Kind == Error {
			//fmt.Printf("ETOK %v\n", t)
			return false
		}
	}
	return true
}
