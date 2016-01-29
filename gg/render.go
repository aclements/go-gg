// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gg

import (
	"io"
	"log"
	"reflect"

	"github.com/aclements/go-gg/generic"
	"github.com/ajstarks/svgo"
)

func (p *Plot) WriteSVG(w io.Writer, width, height int) error {
	// TODO: Perform layout.

	// Set scale ranges.
	for b := range p.aesMap["x"] {
		b.scale.Ranger(NewFloatRanger(0, float64(width)))
		log.Println("x", b.scale.Ranger(nil))
	}
	for b := range p.aesMap["y"] {
		b.scale.Ranger(NewFloatRanger(float64(height), 0))
		log.Println("y", b.scale.Ranger(nil))
	}

	// XXX Default ranges for other things like color.

	// Create rendering environment.
	env := &renderEnv{make(map[*binding]interface{})}

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
	cache map[*binding]interface{}
}

// XXX
//
// All returned slices will be of the same length.
func (e *renderEnv) get(b ...*binding) []interface{} {
	out := make([]interface{}, len(b))
	maxLen := 0

	for i, b1 := range b {
		if mapped := e.cache[b1]; mapped != nil {
			out[i] = mapped
			continue
		}

		rv := reflect.ValueOf(b1.data.Seq())
		rt := reflect.SliceOf(b1.scale.RangeType())
		mapped := reflect.MakeSlice(rt, rv.Len(), rv.Len())
		if rv.Len() > maxLen {
			maxLen = rv.Len()
		}
		for i := 0; i < rv.Len(); i++ {
			m1 := b1.scale.Map(rv.Index(i).Interface())
			mapped.Index(i).Set(reflect.ValueOf(m1))
		}

		out[i] = mapped.Interface()
		e.cache[b1] = out[i]
	}

	// Cycle all of the slices to the maximum length.
	for i, s := range out {
		out[i] = generic.Cycle(s, maxLen)
	}
	return out
}

func (e *renderEnv) getFirst(b *binding) interface{} {
	rv := reflect.ValueOf(b.data.Seq())
	if rv.Len() == 0 {
		return nil
	}
	return b.scale.Map(rv.Index(0).Interface())
}
