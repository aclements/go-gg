// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gg

import (
	"reflect"
	"strconv"

	"github.com/aclements/go-gg/generic"
)

// A Binding combines a set of data values (such as numeric values or
// factors) with a set of scales for mapping them to visual properties
// (such as position or color). A Binding consists of one or more
// groups, where each group has a sequence of data values and a scale.
// Typically, all groups in a Binding will have the same scale, but
// this isn't necessary.
type Binding map[*GroupID]*BindingGroup

// TODO: Maybe call this VarScalar? BindingGroup could be a group of
// Bindings, which it isn't.

// TODO: This representation doesn't force that all groups in a
// binding have the same Var type. If we had a first-class
// representation of a collection of bindings, each binding could just
// be a single Var with an extra "column" giving the groups. Or we
// could require each group to be at a contiguous range of indexes.
// For example, the current representation makes TransformFlatten
// really hard to write.

// A BindingGroup represents a single group within a Binding. It is a
// sequence of data values with a consistent scale.
type BindingGroup struct {
	Var    Var
	Scaler Scaler
}

// TODO: Should GroupID be the pointer, since you never want to
// operate on the struct itself?

type GroupID struct {
	Parent *GroupID
	Label  string
}

var RootGroupID = (*GroupID)(nil)

func (g *GroupID) String() string {
	if g == nil {
		return "/"
	}
	buflen := -1
	for p := g; p != nil; p = p.Parent {
		buflen += len(p.Label) + 1
	}
	buf := make([]byte, buflen)
	bufpos := len(buf)
	for p := g; p != nil; p = p.Parent {
		if p != g {
			bufpos--
			buf[bufpos] = '/'
		}
		bufpos -= len(p.Label)
		copy(buf[bufpos:], p.Label)
	}
	return string(buf)
}

func (g *GroupID) Extend(label string) *GroupID {
	return &GroupID{g, label}
}

// TODO: GroupByKey? Would the key function only work on one binding?
// With a first-class row representation we could pass that.

// TransformGroupBy groups all bindings such that all of the rows in
// each group have equal values for all of the named bindings. Each
// named binding must be at least nominal.
func TransformGroupBy(props ...string) Plotter {
	return func(p *Plot) {
		bindings := make(map[string]Binding)
		for _, name := range p.bindings() {
			bindings[name] = p.getBinding(name)
		}

		for _, prop := range props {
			p.mustGetBinding(prop)
			bindings = group1(bindings, prop)
		}

		p.unbindAll()
		for name, b := range bindings {
			p.Bind(name, b)
		}
	}
}

func group1(bindings map[string]Binding, prop string) map[string]Binding {
	out := make(map[string]Binding)
	for name := range bindings {
		out[name] = make(Binding)
	}

	for gid, bg := range bindings[prop] {
		bgLen := bg.Var.Len()

		// Create index.
		subgroups := make(map[interface{}]*GroupID)
		indexes := make(map[*GroupID][]int)
		v, ok := bg.Var.(VarNominal)
		if !ok {
			panic(&TypeError{"gg.TransformGroupBy", reflect.TypeOf(bg.Var), "must be a VarNominal"})
		}
		seq := reflect.ValueOf(v.Seq())
		for i := 0; i < seq.Len(); i++ {
			x := seq.Index(i).Interface()
			subgid := subgroups[x]
			if subgid == nil {
				subgid = gid.Extend(strconv.FormatInt(int64(len(subgroups)), 10))
				subgroups[x] = subgid
				indexes[subgid] = []int{}
			}
			indexes[subgid] = append(indexes[subgid], i)
		}

		// Split this group in all bindings.
		for subgid, index := range indexes {
			xform := func(seq interface{}) interface{} {
				if reflect.ValueOf(seq).Len() <= 1 {
					return seq
				}

				seq = generic.Cycle(seq, bgLen)
				seq = generic.MultiIndex(seq, index)
				return seq
			}
			for name, b2 := range bindings {
				out[name][subgid] = &BindingGroup{
					Var:    transformVar(b2[gid].Var, xform),
					Scaler: b2[gid].Scaler,
				}
			}
		}
	}

	return out
}

// TransformGroupAuto groups by all bindings that are VarNomimal or
// VarOrdinal, but not VarCardinal.
//
// TODO: Maybe there should be a DiscreteBindings that returns the set
// of discrete bindings, which callers could just pass to
// TransformGroupBy, possibly after manipulating.
func TransformGroupAuto() Plotter {
	return func(p *Plot) {
		discrete := []string{}

		g1 := p.groups()[0]
		for _, name := range p.bindings() {
			v := p.getBinding(name)[g1].Var
			if _, ok := v.(VarNominal); !ok {
				continue
			}
			if _, ok := v.(VarCardinal); ok {
				continue
			}
			discrete = append(discrete, name)
		}

		if len(discrete) > 0 {
			TransformGroupBy(discrete...)(p)
		}
	}
}
