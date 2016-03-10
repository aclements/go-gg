// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gg

import (
	"fmt"
	"reflect"

	"github.com/aclements/go-gg/generic"
	"github.com/aclements/go-gg/table"
)

// TODO: What if there are already layers? Maybe they should be
// repeated in all facets. ggplot2 apparently does this when the
// faceting variable isn't in one of the data frames.

// TODO: If I apply X faceting and then Y faceting, I have to figure
// out that the Y labeling should only apply at the edge and not in
// each column. The same problem applies to sharing scales marks.
//
// I'll see this in the group values, but there I have to be careful
// about faceting that has already happened on the *same* dimension,
// which could result in multiple facets for the same value.

// TODO: FacetWrap

// FacetCommon is the base type for plot faceting operations. Faceting
// is a grouping operation that subdivides a plot into subplots based
// on the values in data column. Faceting operations may be composed:
// if a faceting operation has already divided the plot into subplots,
// a further faceting operation will subdivide each of those subplots.
type FacetCommon struct {
	// Col names the column to facet by. Each distinct value of
	// this column will become a separate plot. If Col is
	// orderable, the facets will be in value order; otherwise,
	// they will be in index order.
	Col string

	// SplitXScales indicates whether plots created by this
	// faceting operation should have separate X axis scales. The
	// default, false, indicates that all of the plots created by
	// splitting up an existing plot should share the X axis of
	// the original plot. If true, each plot will have a separate
	// X axis scale.
	SplitXScales bool

	// SplitYScales is the equivalent of SplitXScales for Y axis
	// scales.
	SplitYScales bool

	// Labeler is a function that constructs facet labels from
	// data values. If this is nil, the default is fmt.Sprint.
	Labeler func(interface{}) string
}

// FacetX splits a plot into columns.
type FacetX FacetCommon

// FacetY splits a plot into rows.
type FacetY FacetCommon

func (f FacetX) Apply(p *Plot) {
	(*FacetCommon)(&f).apply(p, "x")
}

func (f FacetY) Apply(p *Plot) {
	(*FacetCommon)(&f).apply(p, "y")
}

func (f *FacetCommon) apply(p *Plot, dir string) {
	if f.Labeler == nil {
		f.Labeler = func(x interface{}) string { return fmt.Sprint(x) }
	}

	grouped := table.GroupBy(p.Data(), f.Col)

	// TODO: What should this do if there are multiple faceting
	// operations and the results aren't a complete cross-product?

	// TODO: If this is, say, and X faceting and different
	// existing columns have different sets of values, should I
	// only split a column on the values it has? Doing that right
	// would require grouping existing subplots in potentially
	// complex ways (for example, if I do a FacetWrap and then a
	// FacetX, grouping subplots by column alone will be wrong.)
	//
	// I'm going to need to do this grouping anyway to put labels
	// in the right places. Alternatively, I could put labels on
	// every subplot and let final layout eliminate redundant
	// labels that aren't interrupted by a higher-level label.

	// Collect grouped values. If there was already grouping
	// structure, it's possible we'll have multiple groups with
	// the same value for Col.
	type valInfo struct {
		index int
		label string
	}
	var valType reflect.Type
	vals := make(map[interface{}]*valInfo)
	for i, gid := range grouped.Tables() {
		val := gid.Label()
		if _, ok := vals[val]; !ok {
			vals[val] = &valInfo{len(vals), f.Labeler(val)}
		}
		if i == 0 {
			valType = reflect.TypeOf(val)
		}
	}

	// If f.Col is orderable, order and re-index values.
	if generic.CanOrderR(valType.Kind()) {
		valSeq := reflect.MakeSlice(reflect.SliceOf(valType), 0, len(vals))
		for val := range vals {
			valSeq = reflect.Append(valSeq, reflect.ValueOf(val))
		}
		generic.Sort(valSeq.Interface())
		for i := 0; i < valSeq.Len(); i++ {
			vals[valSeq.Index(i).Interface()].index = i
		}
	}

	// Find existing subplots, split existing subplots into
	// len(vals) new subplots, and transform each GroupBy group
	// into its new subplot.
	subplots := make(map[*subplot][]*subplot)
	ndata := table.Grouping(new(table.Table))
	for _, gid := range grouped.Tables() {
		// Find subplot by walking up group hierarchy.
		sub := subplotOf(gid)

		// Split old subplot into len(vals) new subplots.
		nsubplots := subplots[sub]
		if nsubplots == nil {
			nsubplots = make([]*subplot, len(vals))
			for _, val := range vals {
				ns := &subplot{parent: sub}
				if dir == "x" {
					ns.col = val.index
					ns.labelTop = val.label
					ns.showLabelTop = true
				} else {
					ns.row = val.index
					ns.labelRight = val.label
					ns.showLabelRight = true
				}
				nsubplots[val.index] = ns
			}
			subplots[sub] = nsubplots
		}

		// Map this group to its new subplot.
		nsubplot := nsubplots[vals[gid.Label()].index]
		ngid := gid.Parent().Extend(nsubplot)
		ndata = ndata.AddTable(ngid, grouped.Table(gid))
	}

	p.SetData(ndata)

	if f.SplitXScales || f.SplitYScales {
		// TODO
		Warning.Print("not implemented: splitting scales")
	}
}

type subplot struct {
	parent *subplot

	// row and col are the row and column of this subplot in its
	// parent, where 0, 0 is the top left.
	row, col int

	// TODO: Flags for which scale marks to show. Alternatively,
	// if layout eliminates redundant labels, it should probably
	// also figure out which scale marks to show.

	labelTop, labelRight         string
	showLabelTop, showLabelRight bool
}

var rootSubplot = &subplot{}

func subplotOf(gid table.GroupID) *subplot {
	for ; gid != table.RootGroupID; gid = gid.Parent() {
		sub, ok := gid.Label().(*subplot)
		if ok {
			return sub
		}
	}
	return rootSubplot
}

func (s subplot) String() string {
	return fmt.Sprintf("[%d %d]", s.col, s.row)
}
