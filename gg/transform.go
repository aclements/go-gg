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
	// TODO: This is pretty gnarly and has to deal with a lot of
	// conventions simultaneously. This might be much easier if we
	// had a first-class data table representation that we could
	// easily clone and manipulate one binding-group at a time
	// before re-binding the whole data table. That presumably
	// wouldn't track scales, but we could do that by attaching
	// scales to points in the group hierarchy and implicitly
	// inheriting them in descendants. This could also resolve the
	// tension between bindings being *mostly* aesthetic
	// properties, but not entirely.

	return func(p *Plot) {
		// Prepare the new set of bindings.
		newBindings := make(map[string]Binding)
		for _, b2 := range p.bindings() {
			newBindings[b2] = Binding{}
		}

		b := p.mustGetBinding(prop)

		// Sort each group.
		for _, gid := range p.groups() {
			bg := b[gid]

			var bSorter sort.Interface
			switch v := bg.Var.(type) {
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
					seq = generic.Cycle(seq, bg.Var.Len())
					return seq
				}
				for _, name := range p.bindings() {
					b2 := p.getBinding(name)
					nvar := b2[gid].Var
					if nvar.Len() != bg.Var.Len() && nvar.Len() > 1 {
						nvar = transformVar(nvar, xform)
					}
					newBindings[name][gid] = &BindingGroup{
						Var:    nvar,
						Scaler: b2[gid].Scaler,
					}
				}
				continue
			}

			// Generate an initial permutation sequence.
			perm := make([]int, bg.Var.Len())
			for i := range perm {
				perm[i] = i
			}

			// Sort the permutation sequence using b as the key.
			sort.Stable(&permSort{perm, bSorter})

			// Cycle and permute all bindings.
			xform := func(seq interface{}) interface{} {
				seq = generic.Cycle(seq, bg.Var.Len())
				seq = generic.MultiIndex(seq, perm)
				return seq
			}
			for _, name := range p.bindings() {
				b2 := p.getBinding(name)
				nvar := b2[gid].Var

				// No need to cycle or permute constants.
				if nvar.Len() > 1 {
					nvar = transformVar(nvar, xform)
				}

				newBindings[name][gid] = &BindingGroup{
					Var:    nvar,
					Scaler: b2[gid].Scaler,
				}
			}
		}

		// Shadow bindings with sorted bindings.
		for name, b := range newBindings {
			p.Bind(name, b)
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
