// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gg

// LayerLines is like LayerPaths, but connects data points in order by
// the "x" property.
type LayerLines LayerPaths

func (l LayerLines) Apply(p *Plot) {
	defer p.Save().Restore()
	// TODO: This doesn't fill in the default correctly if X is "".
	p = p.SortBy(l.X)
	LayerPaths(l).Apply(p)
}

//go:generate stringer -type StepMode

// StepMode controls how LayerSteps connects subsequent points.
type StepMode int

const (
	// StepHV makes LayerSteps connect subsequent points with a
	// horizontal segment and then a vertical segment.
	StepHV StepMode = iota

	// StepVH makes LayerSteps connect subsequent points with a
	// vertical segment and then a horizontal segment.
	StepVH

	// StepHMid makes LayerSteps connect subsequent points A and B
	// with three segments: a horizontal segment from A to the
	// midpoint between A and B, followed by vertical segment,
	// followed by a horizontal segment from the midpoint to B.
	StepHMid

	// StepVMid makes LayerSteps connect subsequent points A and B
	// with three segments: a vertical segment from A to the
	// midpoint between A and B, followed by horizontal segment,
	// followed by a vertical segment from the midpoint to B.
	StepVMid
)

// LayerSteps is like LayerPaths, but connects data points with a path
// consisting only of horizontal and vertical segments.
type LayerSteps struct {
	LayerPaths

	Step StepMode
}

func (l LayerSteps) Apply(p *Plot) {
	// TODO: Should this also support only showing horizontal or
	// vertical segments?
	//
	// TODO: This could be a data transform instead of a layer.
	// Then it could be used in conjunction with, for example,
	// ribbons.

	l.resolveDefaults()
	p.marks = append(p.marks, plotMark{&markSteps{
		l.Step,
		p.use("x", l.X),
		p.use("y", l.Y),
		p.use("stroke", l.Color),
		p.use("fill", l.Fill),
	}, p.Data().Tables()})
}

// LayerPaths connects successive data points in each group with a
// path and/or a filled polygon.
type LayerPaths struct {
	// X and Y name columns that define the location of each point
	// on the path. If these are empty, they default to the first
	// and second columns, respectively.
	X, Y string

	// Color names a column that defines the color of each line
	// segment in the path. Color defaults to black.
	Color string

	// Fill names a column that defines the fill color of each
	// polygon with vertices at each data point. Only the fill
	// value of the first point in each group is used. Fill
	// defaults to transparent.
	Fill string

	// XXX Perhaps the theme should provide default values for
	// things like "color". That would suggest we need to resolve
	// defaults like that at render time. Possibly a special scale
	// that gets values from the theme could be used to resolve
	// them.
	//
	// XXX strokeOpacity, fillOpacity, strokeWidth, what other
	// properties do SVG strokes have?
	//
	// XXX Should the set of known styling bindings be fixed, and
	// all possible rendering targets have to know what to do with
	// them, or should the rendering target be able to have
	// different styling bindings they understand (presumably with
	// some reasonable base set)? If the renderer can determine
	// the known bindings, we would probably just capture the
	// environment here (and make it so a captured environment
	// does not change) and hand that to the renderer later.
}

func (l *LayerPaths) resolveDefaults() {
	if l.X == "" {
		l.X = "@0"
	}
	if l.Y == "" {
		l.Y = "@1"
	}
}

func (l LayerPaths) Apply(p *Plot) {
	l.resolveDefaults()
	p.marks = append(p.marks, plotMark{&markPath{
		p.use("x", l.X),
		p.use("y", l.Y),
		p.use("stroke", l.Color),
		p.use("fill", l.Fill),
	}, p.Data().Tables()})
}
