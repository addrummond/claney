package compiler

import (
	"bytes"
	"fmt"
	"runtime"
	"sync"
	"unicode/utf8"
)

type node struct {
	epsilons []*node
	mask     [4]uint64
	next     *node
	packed   uint32
}

func (n *node) getBackReachable() bool {
	return n.packed&1 != 0
}

func (n *node) setBackReachable(b bool) {
	if b {
		n.packed |= 1
	} else {
		n.packed &= ^uint32(1)
	}
}

const regexpSpecialChars = "*+?.\\/|()[]"

func regexpToNfa(regexp string) (*node, error) {
	nodePool := make([]node, 0)
	newNode := func() *node {
		nodePool = append(nodePool, node{})
		return &nodePool[len(nodePool)-1]
	}

	starts := make([]*node, 1)
	starts[0] = newNode()
	ends := make([]*node, 1)
	ends[0] = nil
	current := starts[0]
	var prev *node

	for i := 0; i < len(regexp); i++ {
		b := regexp[i]
		switch b {
		case '\\':
			i++
			if i > len(regexp) || !bytes.ContainsAny([]byte{b}, regexpSpecialChars) {
				return nil, fmt.Errorf("bad escape '%c'", b)
			}

			b = regexp[i]
			setMask(&current.mask, b)
			n := newNode()
			current.next = n
			prev = current
			current = n
		case '.':
			allMask(&current.mask)
			n := newNode()
			current.next = n
			prev = current
			current = n
		case '*':
			if i == 0 || regexp[i-1] == '*' || regexp[i-1] == '+' || regexp[i-1] == '?' {
				return nil, fmt.Errorf("invalid *")
			}
			if i+1 < len(regexp) && regexp[i+1] == '?' {
				i++
			}

			newCurrent := newNode()
			prev.setBackReachable(true)
			current.epsilons = append(current.epsilons, prev, newCurrent)
			prev.epsilons = append(prev.epsilons, newCurrent)
			current = newCurrent
		case '+':
			if i == 0 || regexp[i-1] == '*' || regexp[i-1] == '+' || regexp[i-1] == '?' {
				return nil, fmt.Errorf("invalid +")
			}
			if i+1 < len(regexp) && regexp[i+1] == '?' {
				i++
			}

			newCurrent := newNode()
			prev.setBackReachable(true)
			current.epsilons = append(current.epsilons, prev, newCurrent)
			current = newCurrent
		case '?':
			if i == 0 || regexp[i-1] == '*' || regexp[i-1] == '+' || regexp[i-1] == '?' {
				return nil, fmt.Errorf("invalid ?")
			}

			prev.epsilons = append(prev.epsilons, current)
		case '|':
			if ends[len(ends)-1] != nil {
				current.epsilons = append(current.epsilons, ends[len(ends)-1])
			}
			newStart := *starts[len(starts)-1]
			newCurrent := newNode()
			disjunction := starts[len(starts)-1]
			noMask(&disjunction.mask)
			disjunction.next = nil
			disjunction.epsilons = append(disjunction.epsilons, &newStart, newCurrent)
			current = newCurrent
		case '(':
			if i+2 < len(regexp) && regexp[i+1] == '?' && regexp[i+2] == ':' {
				i += 2
			}
			ends = append(ends, newNode())
			starts = append(starts, current)
			prev = nil
		case ')':
			if len(starts) <= 1 {
				return nil, fmt.Errorf("unexpected closing paren")
			}

			current.epsilons = append(current.epsilons, ends[len(ends)-1])
			current = ends[len(ends)-1]
			prev = starts[len(starts)-1]
			ends = ends[:len(starts)-1]
			starts = starts[:len(starts)-1]
		case '[':
			i++
			var mask [4]uint64
			var err error
			mask, i, err = parseRange(regexp, i)
			if err != nil {
				return nil, err
			}

			n := newNode()
			current.next = n
			current.mask = mask
			prev = current
			current = n

		default:
			setMask(&current.mask, b)
			n := newNode()
			current.next = n
			prev = current
			current = n
		}
	}

	return starts[0], nil
}

func parseRange(regexp string, i int) ([4]uint64, int, error) {
	// we start at the char after the opening '['

	var mask [4]uint64

	// We don't need general support for ranges, so just special case '0-9'
	if i+3 < len(regexp) && regexp[i] == '0' && regexp[i+1] == '-' && regexp[i+2] == '9' && regexp[i+3] == ']' {
		i += 3
		mask[0] = 0x3ff000000000000
		return mask, i, nil
	}

	orig := i
	not := false
	chars := make([]rune, 0)
	for {
		if i == len(regexp) {
			return mask, i, fmt.Errorf("unexpected end of regexp after '['")
		}
		if i == orig && regexp[i] == '^' {
			not = true
			i++
		} else if regexp[i] == ']' {
			break
		} else {
			r, l := utf8.DecodeRuneInString(regexp[i:])
			if r == '\\' {
				i++
				if i >= len(regexp) {
					return mask, i, fmt.Errorf("expected char following \\ escape in chargroup")
				}
				r, l = utf8.DecodeRuneInString(regexp[i:])
				chars = append(chars, r)
				i += l
			} else {
				chars = append(chars, r)
				i += l
			}
		}
	}

	if len(chars) == 0 {
		return mask, i, fmt.Errorf("empty [] char class")
	}

	for _, c := range chars {
		if c > 127 {
			return mask, i, fmt.Errorf("only ASCII chars permitted in char classes")
		}
		setMask(&mask, byte(c))
	}

	if not {
		invertMask(&mask)
	}

	return mask, i, nil
}

func run(n *node, input string) bool {
	type state struct {
		i int
		n *node
	}

	states := make(map[state]struct{})
	states[state{0, n}] = struct{}{}

	for {
		newStates := make(map[state]struct{})
		for s := range states {
			if s.n == nil {
				return true
			}

			for _, e := range s.n.epsilons {
				newStates[state{s.i, e}] = struct{}{}
			}

			if s.i >= len(input) {
				if s.n.next == nil && len(s.n.epsilons) == 0 {
					return true
				}
				continue
			}

			b := input[s.i]

			if testMask(&s.n.mask, b) {
				newStates[state{s.i + 1, s.n.next}] = struct{}{}
			}
		}

		if len(newStates) == 0 {
			return false
		}

		states = newStates
	}
}

func overlap(n1, n2 *node) bool {
	type state struct {
		n1, n2 *node
	}

	alreadyVisited := make(map[state]struct{})

	states := []state{{n1, n2}}
	newStates := []state{}

	for {
		addState := func(s state) {
			if s.n1.getBackReachable() || s.n2.getBackReachable() {
				if _, ok := alreadyVisited[s]; !ok {
					alreadyVisited[s] = struct{}{}
					newStates = append(newStates, s)
				}
			} else {
				newStates = append(newStates, s)
			}
		}

		for _, s := range states {
			if s.n1 == nil && s.n2 == nil {
				return true
			}

			// To keep the size of the states dictionary smaller, we only store states
			// that have non-epsilon transitions or that are terminal nodes, and walk
			// to other epsilon-accessible states that have non-epsilon transitions or
			// that are terminal nodes.
			foundTerm := false
			epsilonStep(s.n1, func(e1 *node) iterState {
				return epsilonStep(s.n2, func(e2 *node) iterState {
					if isTerminalNode(e1) && isTerminalNode(e2) {
						foundTerm = true
						return iterBreak
					}

					if !(e1 == s.n1 && e2 == s.n2) {
						// This check is not logically necessary, but helps to keep the list
						// of states smaller. Reducing allocation saves a lot more time than
						// is taken by this logically redundant check.
						if !(len(e1.epsilons) == 0 && len(e2.epsilons) == 0 && e1.mask[0]&e2.mask[0] == 0 && e1.mask[1]&e2.mask[1] == 0 && e1.mask[2]&e2.mask[2] == 0 && e1.mask[3]&e2.mask[3] == 0) {
							addState(state{e1, e2})
						}
					}

					return iterContinue
				})
			})
			if foundTerm {
				return foundTerm
			}

			if s.n1.mask[0]&s.n2.mask[0] != 0 || s.n1.mask[1]&s.n2.mask[1] != 0 || s.n1.mask[2]&s.n2.mask[2] != 0 || s.n1.mask[3]&s.n2.mask[3] != 0 {
				addState(state{s.n1.next, s.n2.next})
			}
		}

		if len(newStates) == 0 {
			return false
		}

		newStates, states = states, newStates
		newStates = newStates[:0]
	}
}

type iterState int

const (
	iterContinue iterState = 0
	iterBreak    iterState = 1
)

func epsilonStep(n *node, f func(n *node) iterState) iterState {
	if hasNonEpsilonProgression(n) || isTerminalNode(n) {
		if f(n) == iterBreak {
			return iterBreak
		}
	}
	for _, e := range n.epsilons {
		if epsilonStep(e, f) == iterBreak {
			return iterBreak
		}
	}
	return iterContinue
}

func hasNonEpsilonProgression(n *node) bool {
	return n.mask[0] != 0 || n.mask[1] != 0 || n.mask[2] != 0 || n.mask[3] != 0
}

func isTerminalNode(n *node) bool {
	return n.next == nil && len(n.epsilons) == 0
}

type overlapIndices struct {
	i1, i2 int
}

func findOverlaps(firstNodes []*node) []overlapIndices {
	indices := make(map[*node]int)
	for i, n := range firstNodes {
		indices[n] = i
	}
	nodeOverlaps := bruteForceOverlapCheck(firstNodes)
	iOverlaps := make([]overlapIndices, len(nodeOverlaps))
	for i := range nodeOverlaps {
		iOverlaps[i].i1 = indices[nodeOverlaps[i].n1]
		iOverlaps[i].i2 = indices[nodeOverlaps[i].n2]
	}

	return iOverlaps
}

type overlapOfNodes struct {
	n1, n2 *node
}

// It's difficult (impossible?) to do better than brute force for a general
// regexp overlap check. In a typical route file, routes can be corralled into
// small groups based on their constant affixes, and these groups can be tested
// independently for intragroup overlaps (see groupbyaffix.go). For this reason
// there is little point in trying to do clever things to optimize this check
// for certain subregular languages.
func bruteForceOverlapCheck(firstNodes []*node) []overlapOfNodes {
	overlaps := make([]overlapOfNodes, 0)
	var overlapMutex sync.Mutex

	nThreads := runtime.NumCPU()
	var wg sync.WaitGroup

	thread := func(start int) {
		defer wg.Done()

		for i := start; i < len(firstNodes); i += nThreads {
			for j := 1; j < len(firstNodes)-i; j++ {
				n1 := firstNodes[i]
				n2 := firstNodes[i+j]
				if overlap(n1, n2) {
					overlapMutex.Lock()
					// append can't panic except possibly for OOM, so we should be ok to
					// do this without wrapping this code in a function block and
					// deferring overlapMutex.Unlock()
					overlaps = append(overlaps, overlapOfNodes{n1, n2})
					overlapMutex.Unlock()
				}
			}
		}
	}

	for i := 0; i < nThreads; i++ {
		wg.Add(1)
		go thread(i)
	}

	wg.Wait()

	return overlaps
}

func testMask(m *[4]uint64, val byte) bool {
	i := val / 64
	shift := val % 64
	return (m[i] >> shift & 1) != 0
}

func noMask(m *[4]uint64) {
	m[0] = 0
	m[1] = 0
	m[2] = 0
	m[3] = 0
}

func allMask(m *[4]uint64) {
	m[0] = 0xFFFFFFFFFFFFFFFF
	m[1] = 0xFFFFFFFFFFFFFFFF
	m[2] = 0xFFFFFFFFFFFFFFFF
	m[3] = 0xFFFFFFFFFFFFFFFF
}

func invertMask(m *[4]uint64) {
	m[0] ^= 0xFFFFFFFFFFFFFFFF
	m[1] ^= 0xFFFFFFFFFFFFFFFF
	m[2] ^= 0xFFFFFFFFFFFFFFFF
	m[3] ^= 0xFFFFFFFFFFFFFFFF
}

func setMask(m *[4]uint64, val byte) {
	i := val / 64
	shift := val % 64
	m[i] |= (1 << shift)
}

func debugPrintNfa(n *node) {
	i := 1
	names := make(map[*node]int)
	names[nil] = -1
	names[n] = 0
	debugPrintNfaHelper(n, names, &i)
	fmt.Printf("%+v\n", names)
}

func debugPrintNfaHelper(n *node, names map[*node]int, i *int) {
	name, ok := names[n]
	if !ok {
		return
	}

	toVisit := make([]*node, 0)

	fmt.Printf("%3d[%p]: ", name, n)
	if n.mask[0] != 0 || n.mask[1] != 0 || n.mask[2] != 0 || n.mask[3] != 0 {
		name, ok := names[n.next]
		if !ok {
			names[n.next] = *i
			name = *i
			*i++
			toVisit = append(toVisit, n.next)
		}
		fmt.Printf(" -> %3d |", name)
	} else {
		fmt.Printf("        |")
	}

	fmt.Printf(" (%v)", maskToLetter(&n.mask))

	for _, e := range n.epsilons {
		name, ok := names[e]
		if !ok {
			names[e] = *i
			name = *i
			*i++
			toVisit = append(toVisit, e)
		}
		fmt.Printf(" %v", name)
	}
	fmt.Printf("\n")

	for _, v := range toVisit {
		debugPrintNfaHelper(v, names, i)
	}
}

// Inefficient, only used for debugging
func maskToLetter(mask *[4]uint64) string {
	for i := 0; i < 255; i++ {
		if testMask(mask, byte(i)) {
			return fmt.Sprintf("%c", i)
		}
	}
	return "''"
}
