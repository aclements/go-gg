// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gg

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// Warning is a logger for reporting conditions that don't prevent the
// production of a plot, but may lead to unexpected results.
var Warning = log.New(os.Stderr, "[gg]", log.Lshortfile)

type Plot struct {
	//facet  facet

	env *plotEnv

	aesMap map[string]map[*BindingGroup]bool
	marks  []marker
	used   map[*BindingGroup]bool
}

func NewPlot() *Plot {
	p := &Plot{
		aesMap: make(map[string]map[*BindingGroup]bool),
		used:   make(map[*BindingGroup]bool),
	}
	return p.Save()
}

type plotEnv struct {
	parent   *plotEnv
	bindings map[string]Binding
	groups   []*GroupID
}

// Bind is shorthand for p.BindWithScale(prop, data, nil). It binds
// "data" to visual property "prop" using a default scale.
func (p *Plot) Bind(prop string, data interface{}) *Plot {
	return p.BindWithScale(prop, data, nil)
}

// BindWithScale binds "data" to visual property "prop", using the
// specified scale to map between data values and visual values.
//
// data may be a Binding, Var, or anything that can be accepted by
// AutoVar. If data is a Binding, its groups must match those of any
// other bindings in the Plot. If data is a Var, the Var will be bound
// in every group (which is initially just the root group) with the
// specified scale. If data is anything else, it will first be
// converted to a Var using AutoVar.
//
// The caller must not modify data after binding it.
//
// XXX Because of things like grouping properties, "prop" isn't
// necessarily a visual property. It's marks that interpret bindings
// as visual properties. To make this weirder, these non-visual
// properties don't have sensible scales, but we force there to be one
// anyway.
//
// scale may be nil. If data is a Binding with a non-nil Scales field,
// those scales will be used. Otherwise, BindWithScale will pick a
// scale. If prop is of the form "𝑎_𝑏" and there is an existing
// binding named "𝑎", this binding will inherit 𝑎's scale. This is
// used to make properties like "y" and "y_2" map to the same scale.
// Otherwise, BindWithScale will chose a default scale based on the
// type of data.
//
// BindWithScale returns p for ease of method chaining.
//
// TODO: This is not commutative, since binding a Binding followed by
// a Var could result in a non-trivial group structure, while binding
// a Var followed by a Binding could be an error.
func (p *Plot) BindWithScale(prop string, data interface{}, scale Scaler) *Plot {
	// Promote data to a Binding.
	var b Binding
	checkGroups := true
	switch data := data.(type) {
	case Binding:
		b = data

	default:
		v := AutoVar(data)
		b = make(map[*GroupID]*BindingGroup)
		for _, gid := range p.groups() {
			b[gid] = &BindingGroup{Var: v}
		}

		// b has the right groups by construction.
		checkGroups = false
	}

	// If this is the first binding, use its group structure.
	if len(p.bindings()) == 0 {
		// This is slightly subtle. Attaching the groups to
		// the inner-most environment is safe because this
		// environment must contain the first binding. This
		// keeps Restoring to an empty environment working and
		// works with unbinding.
		p.env.groups = []*GroupID{}
		for gid := range b {
			p.env.groups = append(p.env.groups, gid)
		}
		checkGroups = false
	}

	// Check that this binding's group structure matches the other
	// bindings.
	if checkGroups {
		badGroups := false
		haveGroups := p.groups()
		if len(haveGroups) != len(b) {
			badGroups = true
		} else {
			for _, gid := range haveGroups {
				if _, ok := b[gid]; !ok {
					badGroups = true
					break
				}
			}
		}
		if badGroups {
			// TODO: More informative error.
			panic(fmt.Sprintf("cannot bind %s; data's groups do not match other bindings", prop))
		}
	}

	// Chose scales.
	needScales := true
	if scale != nil {
		// Override any existing scales in the binding.
		for _, gid := range p.groups() {
			b[gid].Scaler = scale
		}
		needScales = false
	} else {
		// Do we need scales?
		for _, bg := range b {
			if bg.Scaler == nil {
				needScales = true
				break
			}
		}
	}
	if needScales {
		if i := strings.Index(prop, "_"); i >= 0 {
			// Find the scale of the base binding, if any.
			scaleName := prop[:i]
			if b2 := p.getBinding(scaleName); b2 != nil {
				for gid, bg := range b2 {
					if b[gid].Scaler == nil {
						b[gid].Scaler = bg.Scaler
					}
				}
			}
			needScales = false
		}

		if needScales {
			// Choose the default scale for this data.
			for _, bg := range b {
				if bg.Scaler != nil {
					continue
				}
				scale, err := DefaultScale(bg.Var)
				if err != nil {
					panic(err)
				}
				bg.Scaler = scale
			}
		}
	}

	p.env.bindings[prop] = b
	return p
}

func (p *Plot) unbindAll() *Plot {
	for _, name := range p.bindings() {
		p.env.bindings[name] = nil
	}
	p.env.groups = nil
	return p
}

// TODO: Make these public. Might want to just return the name, data,
// and scale rather than the internal struct. OTOH, that would make
// bindings() difficult.

func (p *Plot) getBinding(name string) Binding {
	for e := p.env; e != nil; e = e.parent {
		if b, ok := e.bindings[name]; ok {
			return b
		}
	}
	return nil
}

func (p *Plot) mustGetBinding(name string) Binding {
	if b := p.getBinding(name); b != nil {
		return b
	}
	panic(fmt.Sprintf("unknown binding %q", name))
}

func (p *Plot) bindings() []string {
	bs := make([]string, 0, len(p.env.bindings))
	have := make(map[string]bool)
	for e := p.env; e != nil; e = e.parent {
		for name, b := range e.bindings {
			if !have[name] {
				have[name] = true
				if b != nil {
					bs = append(bs, name)
				}
			}
		}
	}
	return bs
}

func (p *Plot) groups() []*GroupID {
	for e := p.env; e != nil; e = e.parent {
		if e.groups != nil {
			return e.groups
		}
	}
	return []*GroupID{RootGroupID}
}

// use marks a binding as in-use by a mark and attaches it to a
// rendering aesthetic. This adds the binding's data to the domain of
// the binding's scale.
//
// TODO: Should aes be an enum?
func (p *Plot) use(aes string, g *BindingGroup) *Plot {
	// Train the scale.
	if !p.used[g] {
		g.Scaler.ExpandDomain(g.Var)
		p.used[g] = true
	}

	// Add it to the aesthetic mappings.
	am := p.aesMap[aes]
	if am == nil {
		am = make(map[*BindingGroup]bool)
		p.aesMap[aes] = am
	}
	am[g] = true

	return p
}

// Save saves the current bindings of p to a stack. This effectively
// creates a new scope for declaring bindings: all of the existing
// bindings continue to be available, but new bindings can be created
// in the scope and existing bindings can be shadowed by new bindings
// of the same name.
func (p *Plot) Save() *Plot {
	p.env = &plotEnv{parent: p.env, bindings: make(map[string]Binding)}
	return p
}

// Restore restores the bindings of p from the save stack. The
// effectively exits the current bindings scope.
func (p *Plot) Restore() *Plot {
	if p.env.parent == nil {
		panic("unbalanced Save/Restore")
	}
	p.env = p.env.parent
	// TODO: Clear unwound bindings from p.used.
	return p
}

type Plotter func(p *Plot)

func (p *Plot) Add(o Plotter) *Plot {
	o(p)
	return p
}
