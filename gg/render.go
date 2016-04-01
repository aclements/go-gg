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

	"github.com/aclements/go-gg/generic"
	"github.com/aclements/go-gg/table"
	"github.com/ajstarks/svgo"
)

// fontSize is the font size in pixels.
//
// TODO: Theme.
const fontSize float64 = 12

// facetLabelHeight is the height of facet labels, as a multiple of
// the font height.
//
// TODO: Should this be a multiple of fontSize, em height, leading?
// Currently it's leading.
//
// TODO: Theme.
const facetLabelHeight = 1.3

const xTickSep = 5 // TODO: Theme.

const yTickSep = 5 // TODO: Theme.

// plotMargins returns the top, right, bottom, and left margins for a
// plot of the given width and height.
//
// By default, this adds a 5% margin based on the smaller of width and
// height. This ensures that (with automatic scales), the extremes of
// the data and its tick labels don't appear right at the edge of the
// plot area.
//
// TODO: Theme.
var plotMargins = func(w, h float64) (t, r, b, l float64) {
	margin := 0.05 * math.Min(w, h)
	return margin, margin, margin, margin
}

func (p *Plot) WriteSVG(w io.Writer, width, height int) error {
	// TODO: Legend, title.

	// TODO: Check if the same scaler is used for multiple
	// aesthetics with conflicting rangers. Alternatively, if we
	// just computed the scaled data eagerly here, it wouldn't
	// matter if the same Scaler was used for multiple things
	// because we would just change its Ranger between scaling
	// different data. We could still optimize for get/get1 by
	// specifying whether we care about all of the values or just
	// the first when fetching the scaledData (arguably this
	// should also affect scale training, so this is necessary
	// anyway).

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

	// TODO: Automatic aspect ratio by averaging slopes.

	// TODO: Custom tick breaks.

	// TODO: Make sure *all* Scalers have Rangers or the user will
	// get confusing panics.

	// Find all of the subplots and subdivide the marks.
	//
	// TODO: If a mark was done in a parent subplot, broadcast it
	// to all child leafs of that subplot.
	subplots := make(map[*subplot]*eltSubplot)
	plotElts := []plotElt{}
	for _, mark := range p.marks {
		// TODO: This is wrong. If there are non-subplot
		// groups, it'll add the same mark multiple times to
		// the plotElt.
		for _, gid := range mark.groups {
			subplot := subplotOf(gid)
			elt := subplots[subplot]
			if elt == nil {
				elt = newEltSubplot(subplot)
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

	// Add ticks and facet labels.
	plotElts = addSubplotLabels(plotElts)

	// Add axis labels.
	//
	// TODO: Allow user to override these.
	xlabel := strings.Join(generic.Nub(p.axisLabels["x"]).([]string), "\n")
	ylabel := strings.Join(generic.Nub(p.axisLabels["y"]).([]string), "\n")
	plotElts = addAxisLabels(plotElts, xlabel, ylabel)

	// Compute plot element layout.
	layout := layoutPlotElts(plotElts)

	// Perform layout. There's a cyclic dependency involving tick
	// labels here: the tick labels depend on how many ticks there
	// are, how many ticks there are depends on the size of the
	// plot, the size of the plot depends on its surrounding
	// content, and the size of the surrounding content depends on
	// the tick labels. There may not be a fixed point here, so we
	// compromise around the number of ticks.
	//
	// 1) Lay out the graphs without ticks.
	layout.SetLayout(0, 0, float64(width), float64(height))
	// 2) Compute the number of ticks and tick labels.
	const tickDistance = 50 // TODO: Theme. Min pixels between ticks.
	for _, elt := range plotElts {
		elt, ok := elt.(*eltSubplot)
		if !ok {
			continue
		}

		_, _, w, h := elt.Layout()
		genTicks := func(aes string, size float64) {
			for s := range elt.scales[aes] {
				nTicks := int(size / tickDistance)
				if nTicks < 1 {
					nTicks = 1
				}
				major, _, labels := s.Ticks(nTicks)
				elt.addTicks(s, major, labels)
			}
		}
		genTicks("x", w)
		genTicks("y", h)
	}
	// 3) Re-layout the plot and stick with the ticks we computed.
	layout.SetLayout(0, 0, float64(width), float64(height))

	// Draw.
	svg := svg.New(w)
	svg.Start(width, height, fmt.Sprintf(`font-size="%.6gpx" font-family="sans-serif"`, fontSize))
	defer svg.End()

	// Render each plot element.
	for _, elt := range plotElts {
		elt.render(svg)
	}

	return nil
}

func (e *eltSubplot) render(svg *svg.SVG) {
	x, y, w, h := e.Layout()
	m := e.plotMargins

	// Set scale ranges.
	xRanger := NewFloatRanger(x+m.l, x+w-m.r)
	yRanger := NewFloatRanger(y+h-m.b, y+m.t)
	for s := range e.scales["x"] {
		s.Ranger(xRanger)
	}
	for s := range e.scales["y"] {
		s.Ranger(yRanger)
	}

	// Render grid.
	renderBackground(svg, x, y, w, h)
	for s := range e.scales["x"] {
		renderGrid(svg, 'x', s, e.ticks[s], y, y+h)
	}
	for s := range e.scales["y"] {
		renderGrid(svg, 'y', s, e.ticks[s], x, x+w)
	}

	// Create rendering environment.
	env := &renderEnv{cache: make(map[renderCacheKey]table.Slice)}

	// Render marks.
	//
	// TODO: Clip to plot area.
	for _, mark := range e.marks {
		for _, gid := range mark.groups {
			if subplotOf(gid) != e.subplot {
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

	// Skip border and scale ticks.
	//
	// TODO: Theme.
	return

	// Render border.
	r := func(x float64) float64 {
		// Round to nearest N.
		return math.Floor(x + 0.5)
	}
	svg.Path(fmt.Sprintf("M%g %gV%gH%g", r(x), r(y), r(y+h), r(x+w)), "stroke:#888; fill:none; stroke-width:2") // TODO: Theme.

	// Render scale ticks.
	//
	// TODO: These tend not to match up nicely in the
	// bottom left corner. Maybe I need to draw one path
	// for the two lines and then add tick marks.
	for s := range e.scales["x"] {
		renderScale(svg, 'x', s, e.ticks[s], y+h)
	}
	for s := range e.scales["y"] {
		renderScale(svg, 'y', s, e.ticks[s], x)
	}
}

// TODO: Use shape-rendering: crispEdges?

func renderBackground(svg *svg.SVG, x, y, w, h float64) {
	r := func(x float64) int {
		// Round to nearest N.
		return int(math.Floor(x + 0.5))
	}

	svg.Rect(r(x), r(y), r(x+w)-r(x), r(y+h)-r(y), "fill:#eee") // TODO: Theme.
}

func renderGrid(svg *svg.SVG, dir rune, scale Scaler, ticks plotEltTicks, start, end float64) {
	major := mapMany(scale, ticks.major).([]float64)

	r := func(x float64) float64 {
		// Round to nearest N.
		return math.Floor(x + 0.5)
	}

	var path []string
	for _, p := range major {
		if dir == 'x' {
			path = append(path, fmt.Sprintf("M%.6g %.6gv%.6g", r(p), r(start), r(end)-r(start)))
		} else {
			path = append(path, fmt.Sprintf("M%.6g %.6gh%.6g", r(start), r(p), r(end)-r(start)))
		}
	}

	svg.Path(strings.Join(path, ""), "stroke: #fff; stroke-width:2") // TODO: Theme.
}

func renderScale(svg *svg.SVG, dir rune, scale Scaler, ticks plotEltTicks, pos float64) {
	const length float64 = 4 // TODO: Theme

	major := mapMany(scale, ticks.major).([]float64)

	r := func(x float64) float64 {
		// Round to nearest N.
		return math.Floor(x + 0.5)
	}
	var path []string
	for _, p := range major {
		if dir == 'x' {
			path = append(path, fmt.Sprintf("M%.6g %.6gv%.6g", r(p), r(pos), -length))
		} else {
			path = append(path, fmt.Sprintf("M%.6g %.6gh%.6g", r(pos), r(p), length))
		}
	}

	svg.Path(strings.Join(path, ""), "stroke:#888; stroke-width:2") // TODO: Theme
}

func (e *eltTicks) render(svg *svg.SVG) {
	x, y, w, h := e.Layout()
	// TODO: This doesn't show ticks in the margin
	// area. This may be fine with niced tick
	// labels, but it tends to look bad with
	// un-niced ticks.
	m := e.ticksFor.plotMargins
	xRanger := NewFloatRanger(x+m.l, x+w-m.r)
	yRanger := NewFloatRanger(y+h-m.b, y+m.t)
	for s := range e.scales() {
		ticks := e.ticksFor.ticks[s]
		if e.axis == 'x' {
			s.Ranger(xRanger)
		} else {
			s.Ranger(yRanger)
		}
		pos := mapMany(s, ticks.major).([]float64)
		for i, label := range ticks.labels {
			tick := pos[i]
			if e.axis == 'x' {
				svg.Text(int(tick), int(y+xTickSep), label, `text-anchor="middle" dy="1em" fill="#888"`) // TODO: Theme.
			} else {
				svg.Text(int(x+w-yTickSep), int(tick), label, `text-anchor="end" dy=".3em" fill="#888"`)
			}
		}
	}
}

func (e *eltLabel) render(svg *svg.SVG) {
	// TODO: Clip to label region.
	x, y, w, h := e.Layout()
	if e.fill != "none" {
		svg.Rect(int(x), int(y), int(w), int(h), "fill: "+e.fill)
	}
	// Vertical centering is very poorly
	// supported. dy is the best chance.
	style := `text-anchor="middle" dy=".3em"`
	switch e.side {
	case 'l':
		style += fmt.Sprintf(` transform="rotate(-90 %d %d)"`, int(x+w/2), int(y+h/2))
	case 'r':
		style += fmt.Sprintf(` transform="rotate(90 %d %d)"`, int(x+w/2), int(y+h/2))
	}
	svg.Text(int(x+w/2), int(y+h/2), e.label, style)
}

func (e *eltPadding) render(svg *svg.SVG) {
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
	mapped := mapMany(v.scaler, v.seq)
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

func round(x float64) int {
	return int(math.Floor(x + 0.5))
}
