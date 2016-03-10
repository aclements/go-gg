// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gg

import (
	"io"
	"reflect"
	"sort"

	"github.com/aclements/go-gg/gg/layout"
	"github.com/aclements/go-gg/table"
	"github.com/ajstarks/svgo"
)

type subplotInfo struct {
	subplot *subplot
	marks   []plotMark

	rowPath, colPath subplotPath

	// row and col are the global row and column of this subplot.
	row, col int

	layout *layout.Leaf
}

func (si *subplotInfo) getPaths() (rowPath, colPath subplotPath) {
	if si.rowPath == nil {
		var walk func(*subplot) (rp, cp subplotPath)
		walk = func(s *subplot) (rp, cp subplotPath) {
			if s == nil {
				return subplotPath{}, subplotPath{}
			}
			rp, cp = walk(s.parent)
			return append(rp, s.row), append(cp, s.col)
		}
		si.rowPath, si.colPath = walk(si.subplot)
	}
	return si.rowPath, si.colPath
}

type subplotPath []int

func (a subplotPath) cmp(b subplotPath) int {
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

func (p *Plot) WriteSVG(w io.Writer, width, height int) error {
	// TODO: Scales marks, axis labels, facet labels.

	// TODO: Check if the same scaler is used for multiple
	// aesthetics with conflicting rangers.

	// TODO: Rather than finding these scales here and giving them
	// Ratners, we could use special "Width"/"Height" Rangers and
	// assign them much earlier (e.g., when they are Used). We
	// could then either find all of the scales that have those
	// Rangers and configure them at this point, or we could pass
	// the renderEnv in when ranging.

	// TODO: Default ranges for other things like color.

	// TODO: Expose the layout so a package user can put together
	// multiple Plots.

	// Create rendering environment.
	env := &renderEnv{cache: make(map[renderCacheKey]table.Slice)}

	// Find all of the subplots and subdivide the marks.
	//
	// TODO: If a mark was done in a parent subplot, broadcast it
	// to all child leafs of that subplot.
	subplots := make(map[*subplot]*subplotInfo)
	subplotList := []*subplotInfo{}
	for _, mark := range p.marks {
		for _, gid := range mark.groups {
			subplot := subplotOf(gid)
			si := subplots[subplot]
			if si == nil {
				si = &subplotInfo{
					subplot: subplot,
				}
				subplotList = append(subplotList, si)
				subplots[subplot] = si
			}
			si.marks = append(si.marks, mark)
		}
	}

	// Construct the global subplot grid from the grid hierarchy.
	// Do this by sorting the "row path" and "column path" to each
	// leaf and computing global row/col of each leaf.
	//
	// TODO: This isn't very robust to weird faceting. Maybe
	// faceting should be responsible for maintaining a top-level
	// grid.
	sort.Sort(subplotSorter{subplotList, false})
	col, lastPath := -1, subplotPath(nil)
	for _, si := range subplotList {
		if lastPath.cmp(si.colPath) != 0 {
			lastPath = si.colPath
			col++
		}
		si.col = col
	}
	sort.Sort(subplotSorter{subplotList, true})
	row, lastPath := -1, subplotPath(nil)
	for _, si := range subplotList {
		if lastPath.cmp(si.rowPath) != 0 {
			lastPath = si.rowPath
			row++
		}
		si.row = row
	}

	// Construct the grid layout.
	l := new(layout.Grid)
	for _, si := range subplotList {
		si.layout = new(layout.Leaf).SetFlex(true, true)
		l.Add(si.layout, si.col, si.row, 1, 1)
	}
	l.SetLayout(0, 0, float64(width), float64(height))

	svg := svg.New(w)
	svg.Start(width, height)
	defer svg.End()

	// Render each subplot.
	for _, si := range subplotList {
		// Set scale ranges.
		//
		// TODO: Do this only for the scales used by this
		// subplot.
		x, y, w, h := si.layout.Layout()
		for s := range p.scaleSet["x"] {
			s.Ranger(NewFloatRanger(x, x+w))
		}
		for s := range p.scaleSet["y"] {
			s.Ranger(NewFloatRanger(y+h, y))
		}

		// Render marks.
		for _, mark := range si.marks {
			for _, gid := range mark.groups {
				if subplotOf(gid) != si.subplot {
					// TODO: Figure this out when
					// we're building subplots.
					// This is asymptotically
					// inefficient.
					continue
				}
				env.gid = gid
				mark.m.mark(env, svg)
			}
		}
	}

	return nil
}

// subplotSorter sorts subplots by row or column path.
type subplotSorter struct {
	si    []*subplotInfo
	byRow bool
}

func (s subplotSorter) Len() int {
	return len(s.si)
}

func (s subplotSorter) Less(i, j int) bool {
	rowPathI, colPathI := s.si[i].getPaths()
	rowPathJ, colPathJ := s.si[j].getPaths()
	if s.byRow {
		return rowPathI.cmp(rowPathJ) < 0
	}
	return colPathI.cmp(colPathJ) < 0
}

func (s subplotSorter) Swap(i, j int) {
	s.si[i], s.si[j] = s.si[j], s.si[i]
}

type renderEnv struct {
	gid   table.GroupID
	cache map[renderCacheKey]table.Slice
}

type renderCacheKey struct {
	sd  *scaledData
	gid table.GroupID
}

// scaledData is a key for retrieving scaled data from a renderEnv. It
// is the result of using a binding and can be thought of as a lazy
// representation of the visually-mapped data that becomes available
// once all of the scales have been trained.
type scaledData struct {
	seqs map[table.GroupID]scaledSeq
}

type scaledSeq struct {
	seq    table.Slice
	scaler Scaler
}

func (env *renderEnv) get(sd *scaledData) table.Slice {
	cacheKey := renderCacheKey{sd, env.gid}
	if mapped := env.cache[cacheKey]; mapped != nil {
		return mapped
	}

	v := sd.seqs[env.gid]
	rv := reflect.ValueOf(v.seq)
	rt := reflect.SliceOf(v.scaler.RangeType())
	mv := reflect.MakeSlice(rt, rv.Len(), rv.Len())
	for i := 0; i < rv.Len(); i++ {
		m1 := v.scaler.Map(rv.Index(i).Interface())
		mv.Index(i).Set(reflect.ValueOf(m1))
	}

	mapped := mv.Interface()
	env.cache[cacheKey] = mapped
	return mapped
}

func (env *renderEnv) getFirst(sd *scaledData) interface{} {
	if mapped := env.cache[renderCacheKey{sd, env.gid}]; mapped != nil {
		mv := reflect.ValueOf(mapped)
		if mv.Len() == 0 {
			return nil
		}
		return mv.Index(0).Interface()
	}

	v := sd.seqs[env.gid]
	rv := reflect.ValueOf(v.seq)
	if rv.Len() == 0 {
		return nil
	}
	return v.scaler.Map(rv.Index(0).Interface())
}
