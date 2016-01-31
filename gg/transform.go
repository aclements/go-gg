// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gg

import (
	"reflect"
	"sort"

	"github.com/aclements/go-gg/generic"
)

// TransformSort sorts the binding named by prop and permutes all
// other bindings to match the new order of the prop binding. prop
// must not name a VarNominal binding, since nominal variables have no
// order. If bindings differ in length, they are sliced or cycled to
// the length of the prop binding.
func TransformSort(prop string) Plotter {
	return func(p *Plot) {
		b := p.mustGetBinding(prop)

		var bSorter sort.Interface
		switch v := b.data.(type) {
		case VarCardinal:
			bSorter = generic.Sorter(v.Seq())

		case VarOrdinal:
			if l := v.Levels(); l != nil {
				bSorter = sort.IntSlice(l)
			} else {
				bSorter = generic.Sorter(v.Seq())
			}

		default:
			panic(&TypeError{"gg.TransformSort", reflect.TypeOf(v), "cannot sort non-ordinal variable"})
		}
		if sort.IsSorted(bSorter) {
			// Avoid sorting and re-binding everything.
			// Cycle and slice what we have to.
			xform := func(seq interface{}) interface{} {
				seq = generic.Cycle(seq, b.data.Len())
				return seq
			}
			for _, b2 := range p.bindings() {
				if b2.data.Len() != b.data.Len() && b2.data.Len() > 1 {
					nvar := transformVar(b2.data, xform)
					p.BindWithScale(b2.name, nvar, b2.scale)
				}
			}
			return
		}

		// Generate an initial permutation sequence.
		perm := make([]int, b.data.Len())
		for i := range perm {
			perm[i] = i
		}

		// Sort the permutation sequence using b as the key.
		sort.Stable(&permSort{perm, bSorter})

		// Shadow all bindings with cycled and permuted bindings.
		xform := func(seq interface{}) interface{} {
			seq = generic.Cycle(seq, b.data.Len())
			seq = generic.MultiIndex(seq, perm)
			return seq
		}
		for _, b2 := range p.bindings() {
			if b2.data.Len() <= 1 {
				// No need to cycle or permute.
				continue
			}

			nvar := transformVar(b2.data, xform)
			p.BindWithScale(b2.name, nvar, b2.scale)
		}
	}
}

type permSort struct {
	perm []int
	key  sort.Interface
}

func (s *permSort) Len() int {
	return len(s.perm)
}

func (s *permSort) Less(i, j int) bool {
	return s.key.Less(s.perm[i], s.perm[j])
}

func (s *permSort) Swap(i, j int) {
	s.perm[i], s.perm[j] = s.perm[j], s.perm[i]
}
