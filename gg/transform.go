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
// order.
func TransformSort(prop string) Plotter {
	// TODO: This needs to cycle all non-constant bindings to the
	// length of the longest one (or maybe the length of the one
	// being sorted? what if you sort on a constant?).

	return func(p *Plot) {
		b := p.mustGetBinding(prop)
		if b.data.Len() <= 1 {
			// This intentionally includes "constants"
			// even if they aren't ordinal.
			return
		}

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
			panic("cannot sort non-ordinal variable")
		}
		if sort.IsSorted(bSorter) {
			// Avoid sorting and re-binding everything.
			return
		}

		// Generate an initial permutation sequence.
		perm := make([]int, reflect.ValueOf(b.data.(VarNominal).Seq()).Len())
		for i := range perm {
			perm[i] = i
		}

		// Sort the permutation sequence using b as the key.
		sort.Stable(&permSort{perm, bSorter})

		// Shadow all bindings with sorted bindings.
		for _, b := range p.bindings() {
			if b.data.Len() <= 1 {
				continue
			}

			var nvar Var
			switch v := b.data.(type) {
			case VarCardinal:
				nvar = NewVarCardinal(generic.MultiIndex(v.Seq(), perm))

			case VarOrdinal:
				seq := generic.MultiIndex(v.Seq(), perm)
				levels := []int(nil)
				if l := v.Levels(); l != nil {
					levels = generic.MultiIndex(l, perm).([]int)
				}
				nvar = NewVarOrdinal(seq, levels)

			case VarNominal:
				nvar = NewVarNominal(generic.MultiIndex(v.Seq(), perm))

			case Var:
				nvar = NewVar(generic.MultiIndex(v.Seq(), perm))
			}
			p.BindWithScale(b.name, nvar, b.scale)
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
