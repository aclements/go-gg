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
	// TODO: Perform real layout.

	// TODO: Check if the same scaler is used for multiple
	// aesthetics with conflicting rangers.

	// TODO: Rather than finding these scales here and giving them
	// Ratners, we could use special "Width"/"Height" Rangers and
	// assign them much earlier (e.g., when they are Used). We
	// could then either find all of the scales that have those
	// Rangers and configure them at this point, or we could pass
	// the renderEnv in when ranging.

	// TODO: Default ranges for other things like color.

	// Create rendering environment.
	env := &renderEnv{cache: make(map[renderCacheKey]table.Slice)}

	// Split up marks by subplot.
	//
	// TODO: If a mark was done in a parent subplot, broadcast it
	// to all child leafs of that subplot.
	subplots := make(map[*subplot][]plotMark)
	for _, mark := range p.marks {
		for _, gid := range mark.groups {
			subplot := subplotOf(gid)
			subplots[subplot] = append(subplots[subplot], mark)
		}
	}

	svg := svg.New(w)
	svg.Start(width, height)
	defer svg.End()

	// Render each subplot.
	for subplot, marks := range subplots {
		// Set scale ranges.
		//
		// TODO: Do this only for the scales used by this
		// subplot.
		for s := range p.scaleSet["x"] {
			s.Ranger(NewFloatRanger(float64(width)*subplot.l, float64(width)*subplot.r))
		}
		for s := range p.scaleSet["y"] {
			s.Ranger(NewFloatRanger(float64(height)*subplot.b, float64(height)*subplot.t))
		}

		// Render marks.
		for _, mark := range marks {
			for _, gid := range mark.groups {
				if subplotOf(gid) != subplot {
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
	}

	return nil
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
	rv := reflect.ValueOf(v.seq)
	rt := reflect.SliceOf(v.scaler.RangeType())
	mv := reflect.MakeSlice(rt, rv.Len(), rv.Len())
	for i := 0; i < rv.Len(); i++ {
		m1 := v.scaler.Map(rv.Index(i).Interface())
		mv.Index(i).Set(reflect.ValueOf(m1))
	}

	mapped := mv.Interface()
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
