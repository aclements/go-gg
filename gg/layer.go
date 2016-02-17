// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gg

// LayerLines is like LayerPaths, but connects data points in order by
// the "x" property.
func LayerLines() Plotter {
	return func(p *Plot) {
		b := p.mustGetBinding("x")
		if b.isConstant {
			p.Add(LayerPaths())
		} else {
			defer p.Save().Restore()
			p.SortBy(b.col).Add(LayerPaths())
		}
	}
}

// LayerPaths layers a path connecting successive data points in each
// group. By default the path is stroked, but if the "fill" property
// is set, it produces solid polygons.
//
// The "x" and "y" properties define the location of each point on the
// path. They are connected with straight lines.
//
// TODO: Should "x" and "y" default to the first and second columns
// with default scales?
//
// The "color" property defines the color of each line segment in the
// path. If two subsequent points have different color value, the
// color of the first is used. "Color" defaults to black.
//
// The "fill" property defines the fill color of the polygon with
// vertices at each data point. Only the fill value of the first point
// in each group is used. "Fill" defaults to transparent.
//
// XXX Perhaps the theme should provide default values for things like
// "color". That would suggest we need to resolve defaults like that
// at render time. Possibly a special scale that gets values from the
// theme could be used to resolve them.
//
// XXX strokeOpacity, fillOpacity, strokeWidth, what other properties
// do SVG strokes have?
//
// XXX Should the set of known styling bindings be fixed, and all
// possible rendering targets have to know what to do with them, or
// should the rendering target be able to have different styling
// bindings they understand (presumably with some reasonable base
// set)? If the renderer can determine the known bindings, we would
// probably just capture the environment here (and make it so a
// captured environment does not change) and hand that to the renderer
// later.
func LayerPaths() Plotter {
	return func(p *Plot) {
		p.marks = append(p.marks, plotMark{&markPath{
			p.use("x", p.mustGetBinding("x")),
			p.use("y", p.mustGetBinding("y")),
			p.use("stroke", p.getBinding("color")),
			p.use("fill", p.getBinding("fill")),
		}, p.Data().Tables()})
	}
}
