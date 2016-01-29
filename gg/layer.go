// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gg

import "image/color"

// LayerLines is equivalent to LayerPaths, but first sorts the data by
// the "x" variable.
func LayerLines() Plotter {
	return func(p *Plot) {
		p.Save().Add(TransformSort("x")).Add(LayerPaths()).Restore()
	}
}

// LayerPaths layers a path connecting successive data points in each
// group. By default the path is stroked, but if the "fill" property
// is set, it produces solid polygons.
//
// The "x" and "y" properties define the location of each point on the
// path. They are connected with straight lines.
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
		// XXX Groups
		x, y := p.mustGetBinding("x"), p.mustGetBinding("y")
		if x.data.Len() == 0 {
			// XXX Should this be handled at a higher level
			// somehow? If bindings are grouped, we could
			// eliminate zero-length groups.
			return
		}
		if x.data.Len() == 1 {
			// TODO: Depending on the stroke cap, this
			// *could* be well-defined.
			Warning.Print("cannot layer path through 1 point; ignoring")
			return
		}

		colorb := p.getBinding("color")
		if colorb == nil {
			colorb = &binding{name: "color", data: AutoVar(color.Black), scale: NewIdentityScale()}
		}

		fill := p.getBinding("fill")
		if fill == nil {
			fill = &binding{name: "fill", data: AutoVar(color.Transparent), scale: NewIdentityScale()}
		}

		// TODO: Check that scales map to the right types.
		//checkScaleRange("LayerPaths", x, ScaleTypeReal)
		//checkScaleRange("LayerPaths", y, ScaleTypeReal)
		//checkScaleRange("LayerPaths", colorb, ScaleTypeColor)
		//checkScaleRange("LayerPaths", fill, ScaleTypeColor)

		p.use("x", x).use("y", y).use("stroke", colorb).use("fill", fill)
		p.marks = append(p.marks, &markPath{x, y, colorb, fill})
	}
}
