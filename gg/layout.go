// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gg

import (
	"sort"

	"github.com/aclements/go-gg/gg/layout"
)

// textLeading is the height of a line of text.
//
// TODO: Make this real. Chrome's default font-size is 16px, so 20px
// is a fairly comfortable leading.
const textLeading = 20

type eltType int

const (
	eltSubplot eltType = 1 + iota
	eltHLabel
	eltVLabel
)

// A plotElt is a high-level element of a plot layout. It is either a
// subplot body, or a facet label.
type plotElt struct {
	typ eltType

	// For subplot elements.
	subplot *subplot
	marks   []plotMark

	// For label elements.
	label string

	rowPath, colPath eltPath

	// row and col are the global row and column of this element.
	row, col int

	layout *layout.Leaf
}

func newPlotElt(s *subplot) (body *plotElt, labels []*plotElt) {
	var walk func(*subplot) (rp, cp eltPath)
	walk = func(s *subplot) (rp, cp eltPath) {
		if s == nil {
			return eltPath{}, eltPath{}
		}
		rp, cp = walk(s.parent)
		return append(rp, s.row), append(cp, s.col)
	}
	rp, cp := walk(s)
	rp, cp = append(rp, 0), append(cp, 0)

	// Create label elements.
	rightIdx, topIdx := 1, -1
	for s := s; s != nil; s = s.parent {
		if s.showLabelTop {
			l := &plotElt{
				typ:     eltHLabel,
				label:   s.labelTop,
				rowPath: append(rp[:len(rp)-1:len(rp)-1], topIdx),
				colPath: cp,
			}
			labels = append(labels, l)
			topIdx--
		}
		if s.showLabelRight {
			l := &plotElt{
				typ:     eltVLabel,
				label:   s.labelRight,
				rowPath: rp,
				colPath: append(cp[:len(cp)-1:len(cp)-1], rightIdx),
			}
			labels = append(labels, l)
			rightIdx++
		}
	}

	// Create subplot body element.
	body = &plotElt{typ: eltSubplot, subplot: s, rowPath: rp, colPath: cp}

	return
}

type eltPath []int

func (a eltPath) cmp(b eltPath) int {
	for k := 0; k < len(a) && k < len(b); k++ {
		if a[k] != b[k] {
			if a[k] < b[k] {
				return -1
			} else {
				return 1
			}
		}
	}
	if len(a) < len(b) {
		return -1
	} else if len(a) > len(b) {
		return 1
	}
	return 0
}

func layoutPlotElts(elts []*plotElt) layout.Element {
	// Construct the global element grid from the grid hierarchy.
	// Do this by sorting the "row path" and "column path" to each
	// leaf and computing global row/col of each leaf from these
	// orders.
	//
	// TODO: This isn't very robust to weird faceting. Maybe
	// faceting should be responsible for maintaining a top-level
	// grid. Presumably this would work by requiring global
	// row/col in the subplot nodes and considering only the leaf
	// subplot nodes. Perhaps all of the nodes created by a single
	// faceting operation would have a unique tag so I could
	// identify if the leafs were inconsistent. That tag could
	// contain information global to that faceting operation, such
	// as the total number of rows and columns. Each node could
	// belong to a row group and a column group, where these
	// groups are what specify the label for that row/columns and
	// point to the parent row/column group for the next label.
	sort.Sort(plotEltSorter{elts, false})
	col, lastPath := -1, eltPath(nil)
	for _, elt := range elts {
		if lastPath.cmp(elt.colPath) != 0 {
			lastPath = elt.colPath
			col++
		}
		elt.col = col
	}
	sort.Sort(plotEltSorter{elts, true})
	row, lastPath := -1, eltPath(nil)
	for _, elt := range elts {
		if lastPath.cmp(elt.rowPath) != 0 {
			lastPath = elt.rowPath
			row++
		}
		elt.row = row
	}

	// TODO: Merge and eliminate labels. This may be tricky if
	// there's a facet hierarchy on one axis because it's the
	// hierarchy that will be repeated, not just the individual
	// labels.

	// Construct the grid layout.
	l := new(layout.Grid)
	for _, si := range elts {
		si.layout = new(layout.Leaf)
		switch si.typ {
		case eltSubplot:
			si.layout.SetFlex(true, true)
		case eltHLabel:
			si.layout.SetMin(0, textLeading).SetFlex(true, false)
		case eltVLabel:
			si.layout.SetMin(textLeading, 0).SetFlex(false, true)
		}
		l.Add(si.layout, si.col, si.row, 1, 1)
	}
	return l
}

// plotEltSorter sorts plot elements by row or column path.
type plotEltSorter struct {
	si    []*plotElt
	byRow bool
}

func (s plotEltSorter) Len() int {
	return len(s.si)
}

func (s plotEltSorter) Less(i, j int) bool {
	if s.byRow {
		return s.si[i].rowPath.cmp(s.si[j].rowPath) < 0
	}
	return s.si[i].colPath.cmp(s.si[j].colPath) < 0
}

func (s plotEltSorter) Swap(i, j int) {
	s.si[i], s.si[j] = s.si[j], s.si[i]
}
