// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gg

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"reflect"
	"sort"
	"strconv"

	"github.com/aclements/go-gg/generic/slice"
	"github.com/aclements/go-gg/table"
	"github.com/aclements/go-moremath/stats"
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
	var stroke color.Color = color.Black
	if m.stroke != nil {
		stroke = env.getFirst(m.stroke).(color.Color)
	}
	var fill color.Color = color.Transparent
	if m.fill != nil {
		fill = env.getFirst(m.fill).(color.Color)
	}

	drawPath(canvas, xs, ys, stroke, fill)
}

type markSteps struct {
	dir StepMode

	x, y, stroke, fill *scaledData
}

func (m *markSteps) mark(env *renderEnv, canvas *svg.SVG) {
	xs, ys := env.get(m.x).([]float64), env.get(m.y).([]float64)
	var stroke color.Color = color.Black
	if m.stroke != nil {
		stroke = env.getFirst(m.stroke).(color.Color)
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
	}
	if m.dir == StepHV {
		xs2 = xs2[1:]
	} else if m.dir == StepVH {
		ys2 = ys2[1:]
	}

	drawPath(canvas, xs2, ys2, stroke, fill)
}

func drawPath(canvas *svg.SVG, xs, ys []float64, stroke color.Color, fill color.Color) {
	switch len(xs) {
	case 0:
		return
	case 1:
		// TODO: Depending on the stroke cap, this *could* be
		// well-defined.
		Warning.Print("cannot draw path through 1 point; ignoring")
		return
	}

	// Build path.
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
	x, y, color, opacity, size *scaledData
}

func (m *markPoint) mark(env *renderEnv, canvas *svg.SVG) {
	xs, ys := env.get(m.x).([]float64), env.get(m.y).([]float64)
	var colors []color.Color
	if m.color != nil {
		slice.ConvertSlice(&colors, env.get(m.color))
	}
	var opacities []float64
	if m.opacity != nil {
		opacities = env.get(m.opacity).([]float64)
	}
	var sizes []float64
	if m.size != nil {
		sizes = env.get(m.size).([]float64)
	}
	mindim := math.Min(env.Size())

	for i := range xs {
		var style string
		if colors != nil {
			style = cssPaint("fill", colors[i])
		}
		if opacities != nil {
			if style != "" {
				style += ";"
			}
			style += fmt.Sprintf("opacity:%.6g", opacities[i])
		}
		r := mindim * 0.01
		if sizes != nil {
			r = mindim * sizes[i]
		}
		canvas.Circle(int(xs[i]), int(ys[i]), int(r), style)
	}
}

type markTiles struct {
	x, y, fill *scaledData
}

func (m *markTiles) mark(env *renderEnv, canvas *svg.SVG) {
	xs, ys := env.get(m.x).([]float64), env.get(m.y).([]float64)
	// TODO: Should the Scaler (or Ranger) ensure that the values
	// are color.Color? How would this work with an identity
	// scaler?
	var fills []color.Color
	slice.ConvertSlice(&fills, env.get(m.fill))

	// TODO: We can't use an <image> this if the width and height
	// are specified, or if there is a stroke.
	// minx, maxx := stats.Bounds(xs)
	// miny, maxy := stats.Bounds(ys)

	// Compute image bounds.
	imageBounds := func(vals []float64) (float64, float64, float64, bool) {
		// Reduce to unique values.
		unique := []float64{}
		uset := map[float64]bool{}
		for _, v := range vals {
			if !uset[v] {
				if math.IsNaN(v) || math.IsInf(v, 0) {
					continue
				}
				unique = append(unique, v)
				uset[v] = true
			}
		}

		var minGap float64
		regular := true
		switch len(unique) {
		case 0:
			return 0, 0, -1, false
		case 1:
			// TODO: In this case we'll produce a 1 pixel
			// wide/high line. That's probably not what's
			// desired. Maybe we want it to be the
			// width/height of the plot area?
			minGap = 1.0
		default:
			sort.Float64s(unique)
			minGap = unique[1] - unique[0]
			for i, u := range unique[1:] {
				minGap = math.Min(minGap, u-unique[i])
			}
			// Consider the spacing "regular" if every
			// point is within a 1000th of a multiple of
			// minGap.
			for _, u := range unique {
				_, error := math.Modf((u - unique[0]) / minGap)
				if 0.001 <= error && error <= 0.999 {
					regular = false
					break
				}
			}
		}
		return unique[0], unique[len(unique)-1], minGap, regular
	}
	xmin, xmax, xgap, xreg := imageBounds(xs)
	ymin, ymax, ygap, yreg := imageBounds(ys)
	if xgap == -1 || ygap == -1 {
		return
	}
	if !xreg || !yreg {
		// TODO: Can't use an image.
		panic("not implemented: irregular tile spacing")
	}

	// TODO: If there are a small number of cells, just make the
	// rectangles since it's hard to reliably disable
	// interpolation (e.g., the below doesn't work in rsvg).

	// Create the image.
	iw, ih := round((xmax-xmin+xgap)/xgap), round((ymax-ymin+ygap)/ygap)
	img := image.NewRGBA(image.Rect(0, 0, iw, ih))
	for i := range xs {
		img.Set(round((xs[i]-xmin)/xgap), round((ys[i]-ymin)/ygap), fills[i])
	}

	// Encode the image.
	uri := bytes.NewBufferString("data:image/png;base64,")
	w := base64.NewEncoder(base64.StdEncoding, uri)
	if err := png.Encode(w, img); err != nil {
		Warning.Println("error encoding image:", err)
		return
	}
	w.Close()
	canvas.Image(round(xmin-xgap/2), round(ymin-ygap/2),
		round(xmax-xmin+xgap), int(ymax-ymin+ygap),
		uri.String(), `preserveAspectRatio="none" style="image-rendering:optimizeSpeed;image-rendering:-moz-crisp-edges;image-rendering:-webkit-optimize-contrast;image-rendering:pixelated"`)
}

type markTags struct {
	x, y   *scaledData
	labels map[table.GroupID]table.Slice
}

func (m *markTags) mark(env *renderEnv, canvas *svg.SVG) {
	const offsetX float64 = -20
	const offsetY float64 = -20
	const padX float64 = 5

	xs, ys := env.get(m.x).([]float64), env.get(m.y).([]float64)
	if len(xs) == 0 {
		return
	}

	// Find the point closest to the middle.
	//
	// TODO: Give the user control over this.
	minx, maxx := stats.Bounds(xs)
	avgx := (minx + maxx) / 2
	midi, middelta := 0, math.Abs(xs[0]-avgx)
	for i, x := range xs {
		delta := math.Abs(x - avgx)
		if delta < middelta {
			midi, middelta = i, delta
		}
	}

	// Get label.
	label := fmt.Sprint(reflect.ValueOf(m.labels[env.gid]).Index(midi).Interface())

	// Attach tag to this point.
	//
	// TODO: More user control.
	//
	// TODO: Make automatic positioning account for bounds of plot.
	//
	// TODO: Re-enable the tag box when I have decent text metrics.
	//t := measureString(fontSize, label)
	//canvas.Rect(int(xs[midi]+offsetX-t.width), int(ys[midi]+offsetY-0.75*t.leading), int(t.width), int(1.5*t.leading), `rx="4"`, `fill="white"`, `stroke="black"`)
	canvas.Text(int(xs[midi]+offsetX-padX), int(ys[midi]+offsetY), label, `dy=".3em"`, `text-anchor="end"`)
	canvas.Path(fmt.Sprintf("M%.6g %.6gc%.6g %.6g,%.6g %.6g,%.6g %.6g", xs[midi], ys[midi], 0.8*offsetX, 0.0, 0.2*offsetX, offsetY, offsetX, offsetY), `fill="none"`, `stroke="black"`, `stroke-dasharray="2, 3"`, `stroke-width="2"`)
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

	css := prop
	if r>>4 == r&0xF && g>>4 == g&0xF && b>>4 == b&0xF {
		// Use #rgb form.
		css += fmt.Sprintf(":#%x%x%x", r>>4, g>>4, b>>4)
	} else {
		// Use #rrggbb form.
		css += fmt.Sprintf(":#%02x%02x%02x", r, g, b)
	}

	if a != 0xffff {
		// SVG 1.1 only supports CSS2 color formats, which
		// unfortunately does not include rgba, so we have to
		// use a separate CSS property.
		css += ";" + prop + "-opacity:" + strconv.FormatFloat(float64(a)/0xffff, 'g', 0, 64)
	}
	return css
}
