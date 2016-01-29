// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gg

import (
	"image/color"
	"strconv"

	"github.com/ajstarks/svgo"
)

type marker interface {
	mark(env *renderEnv, canvas *svg.SVG)
}

type markPath struct {
	x, y, stroke, fill *binding
}

func (m *markPath) mark(env *renderEnv, canvas *svg.SVG) {
	fill := env.getFirst(m.fill).(color.Color)
	vars := env.get(m.x, m.y, m.stroke)
	// XXX Strokes may not be Gray16, but I don't have a good way
	// to convert from a sequence of things that implement
	// color.Color to a sequence of color.Color.
	xs, ys, strokes := vars[0].([]float64), vars[1].([]float64), vars[2].([]color.Gray16)

	// Is the stroke constant?
	stroke := strokes[0]
	if m.stroke.data.Len() == 1 {
		strokes = nil
	} else {
		for _, s := range strokes {
			if s != stroke {
				goto multistroke
			}
		}
		strokes = nil
	multistroke:
	}

	if strokes == nil {
		// Constant stroke. Use one path.
		path := []byte("M")
		for i := range xs {
			path = append(path, ' ')
			path = strconv.AppendFloat(path, xs[i], 'g', 6, 64)
			path = append(path, ' ')
			path = strconv.AppendFloat(path, ys[i], 'g', 6, 64)
		}

		// XXX Stroke width

		style := cssPaint("stroke", stroke) + ";" + cssPaint("fill", fill)
		canvas.Path(string(path), style)
	} else {
		// Multiple strokes. Use one path for the fill and
		// multiple paths for the stroke.

		// TODO
		panic("multi-color stroke not implemented")
	}
}

// cssPaint returns a CSS fragment for setting CSS property prop to
// color c.
func cssPaint(prop string, c color.Color) string {
	r, g, b, a := c.RGBA()
	if a == 0 {
		// No paint.
		return prop + ":none"
	}

	if a != 0xffff {
		// Undo alpha pre-multiplication.
		r = r * 0xffff / a
		g = g * 0xffff / a
		b = b * 0xffff / a
	}
	r, g, b = r>>8, g>>8, b>>8

	css := prop + ":#"
	if r>>4 == r&0xF && g>>4 == g&0xF && b>>4 == b&0xF {
		// Use #rgb form.
		css += strconv.FormatInt(int64(r>>4), 16) + strconv.FormatInt(int64(g>>4), 16) + strconv.FormatInt(int64(b>>4), 16)
	} else {
		// Use #rrggbb form.
		css += strconv.FormatInt(int64(r), 16) + strconv.FormatInt(int64(g), 16) + strconv.FormatInt(int64(b), 16)
	}

	if a != 0xffff {
		// SVG 1.1 only supports CSS2 color formats, which
		// unfortunately does not include rgba, so we have to
		// use a separate CSS property.
		css += ";" + prop + "-opacity:" + strconv.FormatFloat(float64(a)/0xffff, 'g', 0, 64)
	}
	return css
}
