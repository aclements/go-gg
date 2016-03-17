// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gg

import (
	"fmt"
	"io"
	"math"
	"reflect"
	"strings"

	"github.com/aclements/go-gg/table"
	"github.com/ajstarks/svgo"
)

func (p *Plot) WriteSVG(w io.Writer, width, height int) error {
	// TODO: Scales marks, axis labels, legend.

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

	// TODO: Let the user alternatively specify the width and
	// height of the subplots, rather than the whole plot.

	// TODO: Add margin to plots. This is somewhat tricky because
	// we still want to show ticks in those margins; if we just
	// narrow the ranger, we need a way to ask for the full range
	// of ticks. What we really want is to expand the domain
	// slightly, but what does that mean for discrete scales?

	// Create rendering environment.
	env := &renderEnv{cache: make(map[renderCacheKey]table.Slice)}

	// Find all of the subplots and subdivide the marks.
	//
	// TODO: If a mark was done in a parent subplot, broadcast it
	// to all child leafs of that subplot.
	subplots := make(map[*subplot]*plotElt)
	plotElts := []*plotElt{}
	for _, mark := range p.marks {
		// TODO: This is wrong. If there are non-subplot
		// groups, it'll add the same mark multiple times to
		// the plotElt.
		for _, gid := range mark.groups {
			subplot := subplotOf(gid)
			elt := subplots[subplot]
			if elt == nil {
				elt = newPlotElt(subplot)
				plotElts = append(plotElts, elt)
				subplots[subplot] = elt
			}
			elt.marks = append(elt.marks, mark)
		}
	}
	// Subdivide the scales.
	for sk := range p.scaleSet {
		subplot := subplotOf(sk.gid)
		elt := subplots[subplot]
		if elt == nil {
			continue
		}
		ss := elt.scales[sk.aes]
		if ss == nil {
			ss = make(map[Scaler]bool)
			elt.scales[sk.aes] = ss
		}
		ss[sk.scale] = true
	}

	// Compute plot element layout.
	plotElts = addSubplotLabels(plotElts)
	layoutPlotElts(plotElts).SetLayout(0, 0, float64(width), float64(height))

	// Draw.
	svg := svg.New(w)
	svg.Start(width, height)
	defer svg.End()

	// Render each plot element.
	for _, elt := range plotElts {
		x, y, w, h := elt.layout.Layout()

		if elt.typ == eltHLabel || elt.typ == eltVLabel {
			// TODO: Theme.
			//
			// TODO: Clip to label region.
			svg.Rect(int(x), int(y), int(w), int(h), "fill: #ccc")
			style := `text-anchor="middle"`
			if elt.typ == eltHLabel {
				// Vertical centering is very poorly
				// supported. dy is the best chance.
				style += ` dy=".3em"`
			} else if elt.typ == eltVLabel {
				style += ` writing-mode="tb"`
			}
			svg.Text(int(x+w/2), int(y+h/2), elt.label, style)
			continue
		}

		// Set scale ranges.
		for s := range elt.scales["x"] {
			s.Ranger(NewFloatRanger(x, x+w))
		}
		for s := range elt.scales["y"] {
			s.Ranger(NewFloatRanger(y+h, y))
		}

		// Render marks.
		for _, mark := range elt.marks {
			for _, gid := range mark.groups {
				if subplotOf(gid) != elt.subplot {
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

		// Render scales.
		for s := range elt.scales["x"] {
			renderScale(svg, 'x', s, y+h)
		}
		for s := range elt.scales["y"] {
			renderScale(svg, 'y', s, x)
		}
	}

	return nil
}

func renderScale(svg *svg.SVG, dir rune, scale Scaler, pos float64) {
	const length float64 = 4 // TODO: Theme

	ranger := scale.Ranger(nil)
	p0, p1 := ranger.Map(0).(float64), ranger.Map(1).(float64)
	nTicks := int(math.Abs(p0-p1) / 50)
	if nTicks < 1 {
		nTicks = 1
	}
	major, _ := scale.Ticks(nTicks)

	r := func(x float64) float64 {
		// Round to nearest N.
		return math.Floor(x + 0.5)
	}
	h := func(x float64) float64 {
		// Round to nearest N+0.5.
		return math.Floor(x) + 0.5
	}
	var path []string
	if dir == 'x' {
		path = append(path, fmt.Sprintf("M%.6g %.6gH%.6g", r(p0), h(pos), r(p1)))
	} else {
		path = append(path, fmt.Sprintf("M%.6g %.6gV%.6g", h(pos), r(p0), r(p1)))
	}
	for _, p := range major {
		if dir == 'x' {
			path = append(path, fmt.Sprintf("M%.6g %.6gv%.6g", h(p), r(pos), -length))
		} else {
			path = append(path, fmt.Sprintf("M%.6g %.6gh%.6g", r(pos), h(p), length))
		}
	}

	svg.Path(strings.Join(path, ""), "stroke:#888") // TODO: Theme
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
