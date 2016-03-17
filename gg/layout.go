// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gg

import (
	"sort"

	"github.com/aclements/go-gg/gg/layout"
)

type eltType int

const (
	eltSubplot eltType = 1 + iota
	eltHLabel
	eltVLabel
	eltPadding
)

// A plotElt is a high-level element of a plot layout. It is either a
// subplot body, or a facet label.
//
// plotElts are arranged in a 2D grid. Coordinates in the grid are
// specified by a pair of "paths" rather than a simple pair of
// indexes. For example, element A is to the left of element B if A's
// X path is less than B's X path, where paths are compared as tuples
// with an infinite number of trailing 0's. This makes it easy to, for
// example, place an element to the right of another element without
// having to renumber all of the elements that are already to its
// right.
//
// The first level of the hierarchy is simply the coordinate of the
// plot in the grid. Within this, we layout plot elements as follows:
//
//                +----------------------+
//                | Padding (x, y/-2)    |
//                +----------------------+
//                | HLabel (x, y/-1/-1)  |
//                +----------------------+
//                | Hlabel (x, y/-1/0)   |
//    +-----------+----------------------+------------+----------+
//    | Padding   |                      | VLabel     | Padding  |
//    | (x/-2, y) | Subplot (x, y)       | (x/1/0, y) | (x/2, y) |
//    |           |                      |            |          |
//    +-----------+----------------------+------------+----------+
//                | Padding (x, y/2)     |
//                +----------------------+
type plotElt struct {
	typ            eltType
	xPath, yPath   eltPath // Top left coordinate.
	x2Path, y2Path eltPath // Bottom right. If nil, same as xPath, yPath.

	// For subplot elements.
	subplot *subplot
	marks   []plotMark
	scales  map[string]map[Scaler]bool

	// For label elements.
	label string

	// x, y, xSpan, and ySpan are the global position and span of
	// this element. These are computed by layoutPlotElts.
	x, y         int
	xSpan, ySpan int

	layout *layout.Leaf
}

func newPlotElt(s *subplot) *plotElt {
	return &plotElt{
		typ:     eltSubplot,
		subplot: s,
		scales:  make(map[string]map[Scaler]bool),
		xPath:   eltPath{s.x},
		yPath:   eltPath{s.y},
		layout:  new(layout.Leaf).SetFlex(true, true),
	}
}

func addSubplotLabels(elts []*plotElt) []*plotElt {
	// Find the regions covered by each subplot band.
	type region struct{ x1, x2, y1, y2, level int }
	update := func(r *region, s *subplot, level int) {
		if s.x < r.x1 {
			r.x1 = s.x
		} else if s.x > r.x2 {
			r.x2 = s.x
		}
		if s.y < r.y1 {
			r.y1 = s.y
		} else if s.y > r.y2 {
			r.y2 = s.y
		}
		if level > r.level {
			r.level = level
		}
	}

	vBands := make(map[*subplotBand]region)
	hBands := make(map[*subplotBand]region)
	for _, elt := range elts {
		if elt.typ != eltSubplot {
			continue
		}
		s := elt.subplot

		level := 0
		for vBand := s.vBand; vBand != nil; vBand = vBand.parent {
			r, ok := vBands[vBand]
			if !ok {
				r = region{s.x, s.x, s.y, s.y, level}
			} else {
				update(&r, s, level)
			}
			vBands[vBand] = r
			level++
		}

		level = 0
		for hBand := s.hBand; hBand != nil; hBand = hBand.parent {
			r, ok := hBands[hBand]
			if !ok {
				r = region{s.x, s.x, s.y, s.y, level}
			} else {
				update(&r, s, level)
			}
			hBands[hBand] = r
			level++
		}
	}

	// Create labels.
	textLeading := measureString(fontSize, "").leading
	for vBand, r := range vBands {
		elts = append(elts, &plotElt{
			typ:    eltHLabel,
			label:  vBand.label,
			xPath:  eltPath{r.x1},
			yPath:  eltPath{r.y1, -1, -r.level},
			x2Path: eltPath{r.x2},
			layout: new(layout.Leaf).SetMin(0, textLeading).SetFlex(true, false),
		})
	}
	for hBand, r := range hBands {
		elts = append(elts, &plotElt{
			typ:    eltVLabel,
			label:  hBand.label,
			xPath:  eltPath{r.x2, 1, r.level},
			yPath:  eltPath{r.y1},
			y2Path: eltPath{r.y2},
			layout: new(layout.Leaf).SetMin(textLeading, 0).SetFlex(false, true),
		})
	}
	return elts
}

type eltPath []int

func (a eltPath) cmp(b eltPath) int {
	for len(a) > 0 || len(b) > 0 {
		var ax, bx int
		if len(a) > 0 {
			ax, a = a[0], a[1:]
		}
		if len(b) > 0 {
			bx, b = b[0], b[1:]
		}
		if ax != bx {
			if ax < bx {
				return -1
			} else {
				return 1
			}
		}
	}
	return 0
}

type eltPaths []eltPath

func (s eltPaths) Len() int {
	return len(s)
}

func (s eltPaths) Less(i, j int) bool {
	return s[i].cmp(s[j]) < 0
}

func (s eltPaths) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s eltPaths) nub() eltPaths {
	var i, o int
	for i, o = 1, 1; i < len(s); i++ {
		if s[i].cmp(s[i-1]) != 0 {
			s[o] = s[i]
			o++
		}
	}
	return s[:o]
}

func (s eltPaths) find(p eltPath) int {
	return sort.Search(len(s), func(i int) bool {
		return s[i].cmp(p) >= 0
	})
}

// layoutPlotElts returns a layout containing all of the elements in
// elts.
//
// layoutPlotElts flattens the X and Y paths of elts into simple
// coordinate indexes and constructs a layout.Grid.
func layoutPlotElts(elts []*plotElt) layout.Element {
	const padding = 2 // TODO: Theme.

	// Add padding elements to each subplot.
	//
	// TODO: Should there be padding between labels and the plot?
	for _, elt := range elts {
		if elt.typ != eltSubplot {
			continue
		}
		x, y := elt.xPath[0], elt.yPath[0]
		elts = append(elts,
			// Top.
			&plotElt{typ: eltPadding, xPath: eltPath{x}, yPath: eltPath{y, -2}, layout: new(layout.Leaf).SetMin(0, padding).SetFlex(true, false)},
			// Right.
			&plotElt{typ: eltPadding, xPath: eltPath{x, 2}, yPath: eltPath{y}, layout: new(layout.Leaf).SetMin(padding, 0).SetFlex(false, true)},
			// Bottom.
			&plotElt{typ: eltPadding, xPath: eltPath{x}, yPath: eltPath{y, 2}, layout: new(layout.Leaf).SetMin(0, padding).SetFlex(true, false)},
			// Left.
			&plotElt{typ: eltPadding, xPath: eltPath{x, -2}, yPath: eltPath{y}, layout: new(layout.Leaf).SetMin(padding, 0).SetFlex(false, true)},
		)
	}

	// Construct the global element grid from coordinate paths by
	// sorting the sets of X paths and Y paths to each leaf and
	// computing a global (x,y) for each leaf from these orders.
	dir := func(get func(*plotElt) (p, p2 eltPath, pos, span *int)) {
		var paths eltPaths
		for _, elt := range elts {
			p, p2, _, _ := get(elt)
			paths = append(paths, p)
			if p2 != nil {
				paths = append(paths, p2)
			}
		}
		sort.Sort(paths)
		paths = paths.nub()
		for _, elt := range elts {
			p, p2, pos, span := get(elt)
			*pos = paths.find(p)
			if p2 == nil {
				*span = 1
			} else {
				*span = paths.find(p2) - *pos + 1
			}
		}
	}
	dir(func(e *plotElt) (p, p2 eltPath, pos, span *int) {
		return e.xPath, e.x2Path, &e.x, &e.xSpan
	})
	dir(func(e *plotElt) (p, p2 eltPath, pos, span *int) {
		return e.yPath, e.y2Path, &e.y, &e.ySpan
	})

	// Construct the grid layout.
	l := new(layout.Grid)
	for _, si := range elts {
		l.Add(si.layout, si.x, si.y, si.xSpan, si.ySpan)
	}
	return l
}
