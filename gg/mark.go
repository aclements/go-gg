// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gg

import (
	"image/color"
	"strconv"

	"github.com/aclements/go-gg/table"
	"github.com/ajstarks/svgo"
)

type marker interface {
	mark(env *renderEnv, canvas *svg.SVG)
}

type plotMark struct {
	m      marker
	groups []table.GroupID
}

type markPath struct {
	x, y, stroke, fill *scaledData
}

func (m *markPath) mark(env *renderEnv, canvas *svg.SVG) {
	// XXX What ensures these type assertions will succeed,
	// especially if it's an identity scale? Maybe identity scales
	// still need to coerce their results to the right type.
	xs, ys := env.get(m.x).([]float64), env.get(m.y).([]float64)
	// XXX Strokes may not be Gray16, but I don't have a good way
	// to convert from a sequence of things that implement
	// color.Color to a sequence of color.Color.
	var strokes = []color.Gray16{color.Black}
	if m.stroke != nil {
		strokes = env.get(m.stroke).([]color.Gray16)
	}
	var fill color.Color = color.Transparent
	if m.fill != nil {
		fill = env.getFirst(m.fill).(color.Color)
	}

	drawPath(canvas, xs, ys, strokes, fill)
}

type markSteps struct {
	dir StepMode

	x, y, stroke, fill *scaledData
}

func (m *markSteps) mark(env *renderEnv, canvas *svg.SVG) {
	xs, ys := env.get(m.x).([]float64), env.get(m.y).([]float64)
	// XXX Strokes may not be Gray16.
	var strokes = []color.Gray16{color.Black}
	if m.stroke != nil {
		strokes = env.get(m.stroke).([]color.Gray16)
	}
	var fill color.Color = color.Transparent
	if m.fill != nil {
		fill = env.getFirst(m.fill).(color.Color)
	}

	if len(xs) == 0 {
		return
	}

	// Create intermediate points.
	xs2, ys2 := make([]float64, 2*len(xs)), make([]float64, 2*len(ys))
	var strokes2 []color.Gray16
	if m.stroke == nil {
		strokes2 = strokes
	} else {
		strokes2 = make([]color.Gray16, 2*len(xs))
	}
	for i := range xs2 {
		switch m.dir {
		case StepHV, StepVH:
			xs2[i], ys2[i] = xs[i/2], ys[i/2]
		case StepHMid, StepVMid:
			if i == 0 || i == len(xs2)-1 {
				xs2[i], ys2[i] = xs[i/2], ys[i/2]
				break
			}
			var p1, p2 int
			if i%2 == 0 {
				// Interpolate i/2-1 and i/2.
				p1, p2 = i/2-1, i/2
			} else {
				// Interpolate i/2 and i/2+1.
				p1, p2 = i/2, i/2+1
			}
			if m.dir == StepHMid {
				xs2[i], ys2[i] = (xs[p1]+xs[p2])/2, ys[i/2]
			} else {
				xs2[i], ys2[i] = xs[i/2], (ys[p1]+ys[p2])/2
			}
		}
		if m.stroke != nil {
			strokes2[i] = strokes[i/2]
		}
	}
	if m.dir == StepHV {
		xs2 = xs2[1:]
	} else if m.dir == StepVH {
		ys2 = ys2[1:]
	}
	if m.stroke != nil {
		strokes2 = strokes2[:len(strokes2)-1]
	}

	drawPath(canvas, xs2, ys2, strokes2, fill)
}

func drawPath(canvas *svg.SVG, xs, ys []float64, strokes []color.Gray16, fill color.Color) {
	switch len(xs) {
	case 0:
		return
	case 1:
		// TODO: Depending on the stroke cap, this *could* be
		// well-defined.
		Warning.Print("cannot draw path through 1 point; ignoring")
		return
	}

	// Is the stroke constant?
	stroke := strokes[0]
	for _, s := range strokes {
		if s != stroke {
			Warning.Print("multi-color stroke not implemented")
			break
		}
	}

	// Constant stroke. Use one path.
	path := []byte("M")
	for i := range xs {
		path = append(path, ' ')
		path = strconv.AppendFloat(path, xs[i], 'g', 6, 64)
		path = append(path, ' ')
		path = strconv.AppendFloat(path, ys[i], 'g', 6, 64)
	}

	// XXX Stroke width

	style := cssPaint("stroke", stroke) + ";" + cssPaint("fill", fill) + ";stroke-width:3"
	canvas.Path(string(path), style)
}

type markPoint struct {
	x, y *scaledData
}

func (m *markPoint) mark(env *renderEnv, canvas *svg.SVG) {
	xs, ys := env.get(m.x).([]float64), env.get(m.y).([]float64)

	for i := range xs {
		canvas.Circle(int(xs[i]), int(ys[i]), 4)
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
