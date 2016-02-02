// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gg

import (
	"io"
	"reflect"

	"github.com/aclements/go-gg/generic"
	"github.com/ajstarks/svgo"
)

func (p *Plot) WriteSVG(w io.Writer, width, height int) error {
	// TODO: Perform layout.

	// Set scale ranges.
	for bg := range p.aesMap["x"] {
		bg.Scaler.Ranger(NewFloatRanger(0, float64(width)))
	}
	for bg := range p.aesMap["y"] {
		bg.Scaler.Ranger(NewFloatRanger(float64(height), 0))
	}

	// XXX Default ranges for other things like color.

	// Create rendering environment.
	env := &renderEnv{make(map[*BindingGroup]interface{})}

	// Render.
	svg := svg.New(w)
	svg.Start(width, height)
	for _, mark := range p.marks {
		mark.mark(env, svg)
	}
	svg.End()

	return nil
}

type renderEnv struct {
	cache map[*BindingGroup]interface{}
}

// XXX
//
// All returned slices will be of the same length.
func (e *renderEnv) get(bgs ...*BindingGroup) []interface{} {
	out := make([]interface{}, len(bgs))
	maxLen := 0

	for i, bg := range bgs {
		if mapped := e.cache[bg]; mapped != nil {
			out[i] = mapped
			continue
		}

		rv := reflect.ValueOf(bg.Var.Seq())
		rt := reflect.SliceOf(bg.Scaler.RangeType())
		mapped := reflect.MakeSlice(rt, rv.Len(), rv.Len())
		if rv.Len() > maxLen {
			maxLen = rv.Len()
		}
		for i := 0; i < rv.Len(); i++ {
			m1 := bg.Scaler.Map(rv.Index(i).Interface())
			mapped.Index(i).Set(reflect.ValueOf(m1))
		}

		out[i] = mapped.Interface()
		e.cache[bg] = out[i]
	}

	// Cycle all of the slices to the maximum length.
	for i, s := range out {
		out[i] = generic.Cycle(s, maxLen)
	}
	return out
}

func (e *renderEnv) getFirst(bg *BindingGroup) interface{} {
	rv := reflect.ValueOf(bg.Var.Seq())
	if rv.Len() == 0 {
		return nil
	}
	return bg.Scaler.Map(rv.Index(0).Interface())
}
