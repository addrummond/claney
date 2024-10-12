package compiler

import (
	"sort"
	"strings"
)

type trieNode[Assoc any] struct {
	c        byte
	children map[byte]*trieNode[Assoc]
	assocs   []Assoc
	isWord   bool
}

func addToTrie[Assoc any](pool *[]trieNode[Assoc], root *trieNode[Assoc], s string, assoc Assoc) {
	getNode := func() *trieNode[Assoc] {
		if cap(*pool) <= len(*pool) {
			*pool = make([]trieNode[Assoc], 0, cap(*pool)*2)
		}
		*pool = append(*pool, trieNode[Assoc]{})
		return &((*pool)[len(*pool)-1])
	}

	if len(s) == 0 {
		root.isWord = true
	}
	for i := range s {
		b := s[i]
		if root.children == nil {
			root.children = make(map[byte]*trieNode[Assoc])
		}
		child, ok := root.children[b]
		if !ok {
			child = getNode()
			child.c = b
			root.children[b] = child
		}
		child.isWord = child.isWord || (i+1 == len(s))
		root = child
	}
	root.assocs = append(root.assocs, assoc)
}

func words[Assoc any](node *trieNode[Assoc]) []Assoc {
	accum := make([]Assoc, 0)
	wordsHelper(node, &accum)
	return accum
}

func wordsHelper[Assoc any](node *trieNode[Assoc], accum *[]Assoc) {
	if node.isWord {
		*accum = append(*accum, node.assocs...)
	}

	deterministicChildIter(node, func(c *trieNode[Assoc]) {
		wordsHelper(c, accum)
	})
}

func stoppingPoints[Assoc any](node *trieNode[Assoc]) [][]Assoc {
	accum := make([][]Assoc, 0)
	stoppingPointsHelper(node, node, &accum)

	// Anything associated with the empty prefix should be added to every group
	if node.isWord {
		if len(accum) == 0 {
			accum = append(accum, make([]Assoc, 0))
		}
		for i := range accum {
			accum[i] = append(accum[i], node.assocs...)
		}
	}

	return accum
}

func stoppingPointsHelper[Assoc any](root *trieNode[Assoc], node *trieNode[Assoc], accum *[][]Assoc) {
	if node.isWord && node != root {
		*accum = append(*accum, words(node))
		return
	}

	deterministicChildIter(node, func(c *trieNode[Assoc]) {
		stoppingPointsHelper(root, c, accum)
	})
}

type deterministicChildIterPr[Assoc any] struct {
	b byte
	n *trieNode[Assoc]
}

// Iterating deterministically through the trie is useful for testing, and
// should not have a huge performance impact.
func deterministicChildIter[Assoc any](node *trieNode[Assoc], f func(*trieNode[Assoc])) {
	cs := make([]deterministicChildIterPr[Assoc], 0, 256)
	for i, c := range node.children {
		cs = append(cs, deterministicChildIterPr[Assoc]{i, c})
	}
	sort.Slice(cs, func(i, j int) bool {
		return cs[i].b < cs[j].b
	})
	for _, pr := range cs {
		f(pr.n)
	}
}

func getConstishPrefix(r *CompiledRoute, parents []*CompiledRoute) string {
	var pref strings.Builder
	lastWasSlash := false
	for _, p := range parents {
		writeByteToAffix(&pref, &lastWasSlash, '/')
		writeStringToAffix(&pref, &lastWasSlash, p.Compiled.ConstishPrefix)
		if !allConst(p) {
			return removeTrailingSlash(pref.String())
		}
	}

	writeByteToAffix(&pref, &lastWasSlash, '/')
	writeStringToAffix(&pref, &lastWasSlash, r.Compiled.ConstishPrefix)
	return removeTrailingSlash(pref.String())
}

// split list of routes into non-overlapping groups based on prefix
func groupByConstishPrefix(rwps []RouteWithParents) [][]RouteWithParents {
	return groupByX(rwps, getConstishPrefix)
}

func getConstishSuffix(r *CompiledRoute, parents []*CompiledRoute) string {
	var suff strings.Builder
	lastWashSlash := false
	// This is not unicode correct, but we don't care for this purpose.
	for i := len(r.Compiled.ConstishSuffix) - 1; i >= 0; i-- {
		writeByteToAffix(&suff, &lastWashSlash, r.Compiled.ConstishSuffix[i])
	}
	if allConst(r) {
		writeByteToAffix(&suff, &lastWashSlash, '/')
		for i := len(parents) - 1; i >= 0; i-- {
			p := parents[i]
			// This is not unicode correct, but we don't care for this purpose
			for i := len(p.Compiled.ConstishSuffix) - 1; i >= 0; i-- {
				writeByteToAffix(&suff, &lastWashSlash, p.Compiled.ConstishSuffix[i])
			}
			writeByteToAffix(&suff, &lastWashSlash, '/')
			if !allConst(p) {
				break
			}
		}
	}

	return removeTrailingSlash(suff.String())
}

func removeTrailingSlash(str string) string {
	if len(str) > 1 /* deliberately not zero */ && str[len(str)-1] == '/' {
		return str[:len(str)-1]
	}
	return str
}

// split list of routes into non-overlapping groups based on suffix
func groupByConstishSuffix(rwps []RouteWithParents) [][]RouteWithParents {
	return groupByX(rwps, getConstishSuffix)
}

func allConst(r *CompiledRoute) bool {
	for _, elem := range r.Compiled.Elems {
		if !(elem.kind == slash || elem.kind == constant) {
			return false
		}
	}
	return true
}

func writeByteToAffix(sb *strings.Builder, lastWasSlash *bool, b byte) {
	if b == '/' && (*lastWasSlash || sb.Len() == 0) {
		return
	}
	sb.WriteByte(b)
	*lastWasSlash = (b == '/')
}

func writeStringToAffix(sb *strings.Builder, lastWasSlash *bool, s string) {
	for i := range s {
		b := s[i]

		if b == '/' && (*lastWasSlash || sb.Len() == 0) {
			return
		}
		sb.WriteByte(b)
		*lastWasSlash = (b == '/')
	}
}

func groupByX(rwps []RouteWithParents, getGroupString func(*CompiledRoute, []*CompiledRoute) string) [][]RouteWithParents {
	var trie trieNode[RouteWithParents]
	pool := make([]trieNode[RouteWithParents], 0)

	for _, rwp := range rwps {
		if !rwp.Route.Info.Terminal {
			continue
		}

		groupString := getGroupString(rwp.Route, rwp.Parents)
		addToTrie(&pool, &trie, groupString, rwp)
	}

	return stoppingPoints(&trie)
}
