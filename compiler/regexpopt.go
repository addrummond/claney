package compiler

import (
	"fmt"
	"strings"
)

type renodeKind int

const (
	seq         renodeKind = iota
	group       renodeKind = iota
	nmGroup     renodeKind = iota
	disjunction renodeKind = iota
)

type renode struct {
	kind     renodeKind
	value    string
	children []*renode
}

func parseRegexp(re string) *renode {
	renodePool := make([]renode, 0, len(re)/8+1)

	renodePool = append(renodePool, renode{kind: seq, value: "", children: nil})
	root := &renodePool[len(renodePool)-1]

	current := root
	currentSbStart := 0
	parents := []*renode{}

	getRenode := func(kind renodeKind, value string, children []*renode) *renode {
		if len(renodePool) == cap(renodePool) {
			renodePool = make([]renode, 0, len(renodePool)*2)
		}
		renodePool = append(renodePool, renode{kind, value, children})
		return &renodePool[len(renodePool)-1]
	}

	var i int

	appendLit := func() {
		lit := re[currentSbStart:i]
		current.children = append(current.children, getRenode(seq, lit, nil))
	}

	for i = 0; i < len(re); i++ {
		c := re[i]

		switch c {
		case '(':
			if currentSbStart != i || (len(parents) != 0 && parents[len(parents)-1].kind == disjunction && len(current.children) != 0) {
				appendLit()
			}

			var g *renode
			if i+2 >= len(re) || (re[i+1] != '?' || re[i+2] != ':') {
				g = getRenode(group, "", nil)
			} else {
				i += 2
				g = getRenode(nmGroup, "", nil)
			}

			current.children = append(current.children, g)
			s := getRenode(seq, "", nil)
			g.children = append(g.children, s)
			parents = append(parents, current, g)
			current = s
			currentSbStart = i + 1
		case ')':
			if len(parents) == 0 {
				panic("Bad regexp given to 'parseRegexp' [1]")
			}
			if currentSbStart != i {
				appendLit()
			}
			for len(parents) > 0 && parents[len(parents)-1].kind != group && parents[len(parents)-1].kind != nmGroup {
				parents, current = parents[:len(parents)-1], parents[len(parents)-1]
			}
			if len(parents) < 2 {
				panic("Internal error in 'parseRegexp' [1]")
			}
			parents, current = parents[:len(parents)-2], parents[len(parents)-2]
			currentSbStart = i + 1
		case '|':
			if len(parents) > 0 && parents[len(parents)-1].kind == disjunction {
				if len(current.children) == 0 || currentSbStart != i {
					appendLit()
				}
				n := getRenode(seq, "", nil)
				p := parents[len(parents)-1]
				p.children = append(p.children, n)
				current = n
			} else {
				if len(current.children) == 0 || currentSbStart != i {
					appendLit()
				}

				if current.kind != seq && current.kind != group && current.kind != nmGroup {
					panic(fmt.Sprintf("Internal error in 'parseRegexp': kind is %v", current.kind))
				}

				cp := *current
				current.kind = disjunction
				current.value = ""
				current.children = []*renode{&cp}
				parents = append(parents, current)
				newseq := getRenode(seq, "", nil)
				current.children = append(current.children, newseq)
				current = newseq
			}
			currentSbStart = i + 1
		case '\\':
			i++
			if i >= len(re) {
				panic("Bad regexp given to 'parseRegexp' [2]")
			}
		}
	}

	if currentSbStart != i {
		appendLit()
	}

	return root
}

func writeRegexp(n *renode, sb *strings.Builder) {
	sb.WriteString(n.value)
	switch n.kind {
	case seq:
		for _, c := range n.children {
			writeRegexp(c, sb)
		}
	case group:
		sb.WriteByte('(')
		for _, c := range n.children {
			writeRegexp(c, sb)
		}
		sb.WriteByte(')')
	case nmGroup:
		sb.WriteString("(?:")
		for _, c := range n.children {
			writeRegexp(c, sb)
		}
		sb.WriteByte(')')
	case disjunction:
		for i, c := range n.children {
			if i != 0 {
				sb.WriteByte('|')
			}
			writeRegexp(c, sb)
		}
	}
}

func renodeToString(n *renode) string {
	var sb strings.Builder
	writeRegexp(n, &sb)
	return sb.String()
}

type singleGroupDisjunct struct {
	n                 *renode
	sgsByTrailing     map[string][]*renode
	sgsByTrailingKeys []string // for above map, for determinism
	nsgs              []*renode
}

func findSingleGroupDisjunctsHelper(n *renode, accum *[]singleGroupDisjunct, scratchBuffer []byte) {
	if n.kind == disjunction {
		groupCount := 0
		for _, c := range n.children {
			if isGroupChild(c) {
				groupCount++
			}
		}

		if groupCount >= 3 {
			sgsByTrailing := make(map[string][]*renode)
			sgsByTrailingKeys := make([]string, 0)
			var nsgs []*renode
			for _, c := range n.children {
				if isGroupChild(c) {
					trailing, ok := groupChildTrailing(c, scratchBuffer)
					if !ok {
						nsgs = append(nsgs, c)
					} else {
						if _, ok := sgsByTrailing[trailing]; !ok {
							sgsByTrailingKeys = append(sgsByTrailingKeys, trailing)
						}
						sgsByTrailing[trailing] = append(sgsByTrailing[trailing], c)
					}
				} else {
					nsgs = append(nsgs, c)
				}
			}
			*accum = append(*accum, singleGroupDisjunct{n, sgsByTrailing, sgsByTrailingKeys, nsgs})
		}
	}

	for _, c := range n.children {
		findSingleGroupDisjunctsHelper(c, accum, scratchBuffer)
	}
}

func findSingleGroupDisjuncts(n *renode, scratchBuffer []byte) []singleGroupDisjunct {
	var accum []singleGroupDisjunct
	findSingleGroupDisjunctsHelper(n, &accum, scratchBuffer)
	return accum
}

func isGroupChild(child *renode) bool {
	if child.kind == seq && len(child.children) >= 1 && child.children[0].kind == group && len(child.children[0].children) == 1 && child.children[0].children[0].kind == seq {
		for i := 1; i < len(child.children); i++ {
			if child.children[i].kind != seq {
				return false
			}
		}
		return true
	}
	return false
}

func groupChildTrailing(child *renode, scratchBuffer []byte) (string, bool) {
	if len(child.children) > 4 {
		return "", false
	}

	bufi := 0
	for i := 1; i < len(child.children); i++ {
		c := child.children[i]
		if c.kind != seq {
			return "", false
		}
		if len(scratchBuffer)-bufi < len(c.value) {
			return "", false
		}
		copy(scratchBuffer[bufi:], c.value)
		bufi += len(c.value)
	}

	return string(scratchBuffer[:bufi]), true
}

func refactorSingleGroupDisjuncts(sgds []singleGroupDisjunct) {
	renodePool := make([]renode, 8)

	getRenode := func(kind renodeKind, value string, children []*renode) *renode {
		if len(renodePool) == cap(renodePool) {
			renodePool = make([]renode, 0, len(renodePool)*2)
		}
		renodePool = append(renodePool, renode{kind, value, children})
		return &renodePool[len(renodePool)-1]
	}

	for _, sgd := range sgds {
		disjNsgs := getRenode(disjunction, "", sgd.nsgs)

		sgd.n.kind = disjunction
		sgd.n.value = ""

		sgd.n.children = []*renode{}

		for _, k := range sgd.sgsByTrailingKeys {
			sgs := sgd.sgsByTrailing[k]

			disjSgs := getRenode(disjunction, "", sgs)
			group := getRenode(group, "", []*renode{disjSgs})

			if len(sgs[0].children) == 1 {
				sgd.n.children = append(sgd.n.children, group)
			} else {
				seq := getRenode(seq, "", []*renode{group})
				seq.children = append(seq.children, sgs[0].children[1:]...)
				sgd.n.children = append(sgd.n.children, seq)
			}
		}

		if len(disjNsgs.children) > 0 {
			sgd.n.children = append(sgd.n.children, disjNsgs)
		}

		for _, sgs := range sgd.sgsByTrailing {
			for i := range sgs {
				sgs[i] = sgs[i].children[0].children[0]
			}
		}
	}
}

func debugPrintRenode(n *renode) string {
	const idt = "··"
	var sb strings.Builder

	var lastWasNewline bool
	writeByte := func(c byte) {
		if c != '\n' || !lastWasNewline {
			sb.WriteByte(c)
		}
		lastWasNewline = c == '\n'
	}

	writeString := func(s string) {
		i := 0
		for ; i < len(s) && s[i] == '\n'; i++ {
		}
		if i < len(s) {
			sb.WriteString(s[i:])
			lastWasNewline = s[len(s)-1] == '\n'
		} else {
			lastWasNewline = true
		}
	}

	var rec func(n *renode, indent int)
	rec = func(n *renode, indent int) {
		switch n.kind {
		case seq:
			if len(n.value) > 0 {
				for i := 0; i < indent; i++ {
					writeString(idt)
				}
				writeString(n.value)
			} else if len(n.value) == 0 && len(n.children) == 0 {
				for i := 0; i < indent; i++ {
					sb.WriteString(idt)
				}
				writeString("''\n")
			} else {
				for i := 0; i < indent; i++ {
					writeString(idt)
				}
				writeString(">>\n")
				for _, c := range n.children {
					rec(c, indent+1)
					writeByte('\n')
				}
			}
		case group, nmGroup:
			for i := 0; i < indent; i++ {
				writeString(idt)
			}
			writeByte('(')
			if n.kind == nmGroup {
				writeString("?:")
			}
			writeByte('\n')
			for _, c := range n.children {
				rec(c, indent+1)
				writeByte('\n')
			}
			for i := 0; i < indent; i++ {
				writeString(idt)
			}
			writeString(")")
		case disjunction:
			for i := 0; i < indent; i++ {
				writeString(idt)
			}
			writeString("|(\n")
			for _, c := range n.children {
				rec(c, indent+1)
			}
			for i := 0; i < indent; i++ {
				writeString(idt)
			}
			writeString("|)\n")
		}
	}

	rec(n, 0)
	return sb.String()
}
