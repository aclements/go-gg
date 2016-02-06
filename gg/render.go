// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gg

import (
	"io"
	"reflect"

	"github.com/aclements/go-gg/table"
	"github.com/ajstarks/svgo"
)

func (p *Plot) WriteSVG(w io.Writer, width, height int) error {
	// TODO: Perform layout.

	// TODO: Check if the same scaler is used for multiple
	// aesthetics with conflicting rangers.

	// Set scale ranges.
	for vis := range p.aesMap["x"] {
		vis.scaler.Ranger(NewFloatRanger(0, float64(width)))
	}
	for vis := range p.aesMap["y"] {
		vis.scaler.Ranger(NewFloatRanger(float64(height), 0))
	}

	// XXX Default ranges for other things like color.

	// Create rendering environment.
	env := &renderEnv{make(map[*visual]table.Slice)}

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
	cache map[*visual]table.Slice
}

type visual struct {
	seq    table.Slice
	scaler Scaler
}

func (v *visual) get(env *renderEnv) table.Slice {
	if mapped := env.cache[v]; mapped != nil {
		return mapped
	}

	rv := reflect.ValueOf(v.seq)
	rt := reflect.SliceOf(v.scaler.RangeType())
	mv := reflect.MakeSlice(rt, rv.Len(), rv.Len())
	for i := 0; i < rv.Len(); i++ {
		m1 := v.scaler.Map(rv.Index(i).Interface())
		mv.Index(i).Set(reflect.ValueOf(m1))
	}

	mapped := mv.Interface()
	env.cache[v] = mapped
	return mapped
}

func (v *visual) getFirst(env *renderEnv) interface{} {
	if mapped := env.cache[v]; mapped != nil {
		mv := reflect.ValueOf(mapped)
		if mv.Len() == 0 {
			return nil
		}
		return mv.Index(0).Interface()
	}

	rv := reflect.ValueOf(v.seq)
	if rv.Len() == 0 {
		return nil
	}
	return v.scaler.Map(rv.Index(0).Interface())
}
